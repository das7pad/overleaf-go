// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"context"
	"io"
	"math"
	"os/exec"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type Worker interface {
	Count() int
	Done() <-chan error
	Error() error
	IsReady() bool
	Language() types.SpellCheckLanguage
	CheckWords(ctx context.Context, words []string) ([]string, error)
	Kill(reason error)
	Shutdown(reason error)
	TransitionState(from, to string) bool
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
	done := make(chan error)

	w := worker{
		state:    Ready,
		cmd:      cmd,
		count:    0,
		language: language,
		stdin:    stdinPipe,
		scanner:  scanner,
		done:     done,
	}

	if err = w.start(); err != nil {
		close(done)
		return nil, err
	}
	return &w, nil
}

const (
	Busy    = "busy"
	Closing = "closing"
	End     = "end"
	Error   = "error"
	Killed  = "killed"
	Ready   = "ready"

	// BatchSize Send words in chunks of n.
	BatchSize = 100
)

var (
	errPipeClosedBeforeFinish = errors.New("pipe closed before read finished")
	errProcessExisted         = errors.New("process exited")
)

type worker struct {
	cmd      *exec.Cmd
	count    int
	done     chan error
	err      error
	language types.SpellCheckLanguage
	scanner  *bufio.Scanner
	state    string
	stdin    io.WriteCloser
}

func (w *worker) Count() int {
	return w.count
}

func (w *worker) Done() <-chan error {
	return w.done
}

func (w *worker) Error() error {
	return w.err
}

func (w *worker) Language() types.SpellCheckLanguage {
	return w.language
}

func (w *worker) IsReady() bool {
	return w.state == Ready
}

func (w *worker) CheckWords(ctx context.Context, words []string) ([]string, error) {
	// Cancel the context once any sub-task errored or the parent ctx errored.
	eg, writerContext := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := w.startBatch(); err != nil {
			return err
		}
		// Write until we are done or something errored.
		for len(words) > 0 && writerContext.Err() == nil {
			sizeOfWords := len(words)
			sliceAt := int(math.Min(float64(sizeOfWords), BatchSize))
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

	out := make([]string, 0)
	hasReadEndOfBatchMarker := make(chan bool)
	eg.Go(func() error {
		for w.scanner.Scan() {
			line := w.scanner.Text()
			if line == string(w.language) {
				hasReadEndOfBatchMarker <- true
				return nil
			}
			out = append(out, line)
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
		case processError := <-w.done:
			return processError
		case <-hasReadEndOfBatchMarker:
			return nil
		}
	})

	if err := eg.Wait(); err != nil {
		w.updateState(Error, err)
		w.Kill(err)
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		// Check the parent context for cancelled state.
		return nil, err
	}
	return out, nil
}

func (w *worker) Kill(reason error) {
	w.updateState(Killed, reason)

	// Force-fully exit.
	_ = w.cmd.Process.Kill()
}

func (w *worker) Shutdown(reason error) {
	w.updateState(Closing, reason)

	// Gracefully exit in closing the pipe.
	_ = w.stdin.Close()
}

func (w *worker) TransitionState(from, to string) bool {
	if w.state != from {
		return false
	}
	w.updateState(to, nil)
	return true
}

func (w *worker) endBatch() error {
	// Emit language as delimiter for batches
	return w.write("$$l")
}

func (w *worker) start() error {
	if err := w.cmd.Start(); err != nil {
		return err
	}
	go func() {
		exitError := w.cmd.Wait()
		var state string
		if exitError == nil {
			state = End
			exitError = errProcessExisted
		} else {
			state = Killed
		}
		w.updateState(state, exitError)
		w.done <- w.err
		close(w.done)
	}()
	return nil
}

func (w *worker) startBatch() error {
	// Enter terse mode
	return w.write("!")
}

func (w *worker) sendWords(words []string) error {
	w.count++
	return w.write("^" + strings.Join(words, " "))
}

func (w *worker) write(line string) error {
	_, err := w.stdin.Write([]byte(line + "\n"))
	return err
}

func (w *worker) updateState(state string, err error) {
	if w.err == nil {
		w.state = state
		w.err = err
	}
}
