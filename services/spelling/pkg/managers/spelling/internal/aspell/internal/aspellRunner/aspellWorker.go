// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package aspellRunner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type Worker interface {
	Count() int
	LastUsed() time.Time
	Language() types.SpellCheckLanguage
	CheckWords(ctx context.Context, words []string) (Suggestions, error)
	Kill()
}

func newAspellWorker(language types.SpellCheckLanguage) (Worker, error) {
	if err := language.Validate(); err != nil {
		return nil, err
	}
	cmd := exec.Command(
		"aspell",
		"pipe", "--mode=tex", "--encoding=utf-8", "--master", string(language),
	)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	// Do not leak memory, nothing is reading this on happy path.
	cmd.Stderr = nil

	scanner := bufio.NewScanner(stdoutPipe)

	w := worker{
		cmd:      cmd,
		count:    0,
		language: language,
		stdin:    stdinPipe,
		scanner:  scanner,
		done:     make(chan error),
	}

	if err = w.start(); err != nil {
		return nil, err
	}
	scanner.Scan() // discard aspell version header
	return &w, nil
}

const (
	// BatchSize Send words in chunks of n.
	BatchSize = 100
)

var (
	errPipeClosedBeforeFinish = errors.New("pipe closed before read finished")
	errProcessExisted         = errors.New("process exited")
	spaceSeparator            = []byte(" ")
)

type worker struct {
	language types.SpellCheckLanguage
	lastUsed time.Time
	count    int
	done     chan error
	cmd      *exec.Cmd
	scanner  *bufio.Scanner
	stdin    io.WriteCloser
}

func (w *worker) String() string {
	return fmt.Sprintf("%s:%d", w.language, w.count)
}

func (w *worker) Count() int {
	return w.count
}

func (w *worker) Language() types.SpellCheckLanguage {
	return w.language
}

func (w *worker) LastUsed() time.Time {
	return w.lastUsed
}

func (w *worker) CheckWords(ctx context.Context, words []string) (Suggestions, error) {
	ctx, cancel := context.WithTimeout(ctx, MaxRequestDuration)
	defer cancel()
	// Cancel the context once any sub-task errored or the parent ctx errored.
	eg, writerContext := errgroup.WithContext(ctx)

	out := make(Suggestions, len(words))
	eg.Go(func() error {
		if err := w.startBatch(); err != nil {
			return err
		}
		for len(words) > 0 {
			sliceAt := int(math.Min(float64(len(words)), BatchSize))
			chunk := words[:sliceAt]
			words = words[sliceAt:]

			if err := w.sendWords(chunk); err != nil {
				return err
			}
		}
		if err := w.endBatch(); err != nil {
			return err
		}
		return nil
	})

	hasReadEndOfBatchMarker := make(chan struct{})
	eg.Go(func() error {
		defer close(hasReadEndOfBatchMarker)
		for w.scanner.Scan() {
			b := w.scanner.Bytes()
			if len(b) == 0 {
				continue
			}
			parts := bytes.SplitN(b, spaceSeparator, 5)

			hasSuggestions := len(parts) == 5 &&
				len(parts[0]) == 1 && parts[0][0] == '&' &&
				len(parts[3]) > 1 && parts[3][len(parts[3])-1] == ':'
			if hasSuggestions {
				out[string(parts[1])] = strings.Split(string(parts[4]), ", ")
				continue
			}

			hasNoSuggestions := len(parts) == 3 &&
				len(parts[0]) == 1 && parts[0][0] == '#'
			if hasNoSuggestions {
				out[string(parts[1])] = nil
				continue
			}

			if len(parts) == 1 && string(parts[0]) == string(w.language) {
				return nil
			}
		}
		scanErr := w.scanner.Err()
		if scanErr == nil {
			// Scanner.Err() returns nil in case of EOF.
			scanErr = errPipeClosedBeforeFinish
		}
		return scanErr
	})

	eg.Go(func() error {
		select {
		case <-writerContext.Done():
			if ctx.Err() == nil {
				// Something went wrong with the sub-process.
				return writerContext.Err()
			}
			// The parent context was cancelled.
			// Assumption: Starting a new sub-process with fresh cache incurs a
			//              penalty of 10ms for the next batch.
			// Try to flush the pipeline in 10ms and keep this worker alive.
			t := time.NewTimer(time.Millisecond * 10)
			select {
			case <-t.C:
				w.Kill()
			case processError := <-w.done:
				if !t.Stop() {
					<-t.C
				}
				return processError
			case <-hasReadEndOfBatchMarker:
				if !t.Stop() {
					<-t.C
				}
				return nil
			}
			return writerContext.Err()
		case processError := <-w.done:
			return processError
		case <-hasReadEndOfBatchMarker:
			return nil
		}
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	w.lastUsed = time.Now()
	return out, nil
}

func (w *worker) Kill() {
	_ = w.cmd.Process.Kill()
	<-w.done
}

func (w *worker) endBatch() error {
	// Emit language as delimiter for batches
	return w.write([]byte("$$l\n"))
}

func (w *worker) start() error {
	if err := w.cmd.Start(); err != nil {
		close(w.done)
		return err
	}
	go func() {
		err := w.cmd.Wait()
		if err == nil {
			err = errProcessExisted
		}
		w.done <- err
		close(w.done)
	}()
	return nil
}

func (w *worker) startBatch() error {
	// Enter terse mode
	return w.write([]byte("!\n"))
}

func (w *worker) sendWords(words []string) error {
	w.count++
	var b bytes.Buffer
	n := 1 + len(words) - 1 + 1 // ^ + spaces between words + \n
	for _, item := range words {
		n += len(item)
	}
	b.Grow(n)
	b.WriteString("^")
	b.WriteString(words[0])
	for _, item := range words[1:] {
		b.WriteString(" ")
		b.WriteString(item)
	}
	b.WriteString("\n")
	return w.write(b.Bytes())
}

func (w *worker) write(p []byte) error {
	_, err := w.stdin.Write(p)
	return err
}
