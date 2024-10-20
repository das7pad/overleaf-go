// Golang port of Overleaf
// Copyright (C) 2024 Jakob Ackermann <das7pad@outlook.com>
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

package pdfCaching

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/das7pad/overleaf-go/pkg/copyFile"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Manager interface {
	Process(ctx context.Context, co *types.CompileOptions, namespace types.Namespace, contentId, buildId types.BuildId) ([]types.PDFCachingRange, error)
}

func New(options *types.Options) Manager {
	return &manager{
		outputBaseDir:  options.OutputBaseDir,
		compileBaseDir: options.CompileBaseDir,
	}
}

type manager struct {
	outputBaseDir  types.OutputBaseDir
	compileBaseDir types.CompileBaseDir
}

func (m *manager) Process(ctx context.Context, co *types.CompileOptions, namespace types.Namespace, contentId, buildId types.BuildId) ([]types.PDFCachingRange, error) {
	o := m.outputBaseDir.OutputDir(namespace)
	t := tracker{
		o:          o,
		contentDir: o.ContentDir(contentId),
	}
	t.open()

	if t.LastBuild == buildId {
		return t.LastRanges, nil
	}

	ctx, done := context.WithTimeout(ctx, 10*time.Second)
	defer done()
	r, err := parseXref(
		m.compileBaseDir.CompileDir(namespace).Join(pdfXrefName),
	)
	if err != nil {
		return nil, errors.Tag(err, "xref")
	}
	defer t.flush()
	r, err = extractRanges(ctx, o.CompileOutputDir(buildId).Join(pdfName), &t, r)
	if err != nil {
		t.LastBuild = ""
		t.LastRanges = nil
		return r, err
	}
	t.LastBuild = buildId
	t.LastRanges = r
	return r, nil
}

func extractRanges(ctx context.Context, src string, t *tracker, r []types.PDFCachingRange) ([]types.PDFCachingRange, error) {
	sort.Slice(r, func(i, j int) bool {
		return r[i].Start < r[j].Start
	})
	f, err := os.OpenFile(src, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, errors.Tag(err, "open")
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 100)
	sha := sha256.New()

	r2 := r
	r = r[:0]
	for i, cr := range r2[:len(r2)-1] {
		cr.End = r2[i+1].Start
		n := int64(cr.End - cr.Start)
		if n < minChunkSize {
			continue
		}
		if err = ctx.Err(); err != nil {
			return r, err
		}
		sha.Reset()
		if _, err = f.Seek(int64(cr.Start), io.SeekStart); err != nil {
			return r, errors.Tag(err, "seek")
		}
		buf = buf[:100]
		if _, err = io.ReadFull(f, buf); err != nil {
			return r, errors.Tag(err, "peek")
		}
		idx := int64(bytes.Index(buf, objToken))
		if idx == -1 {
			return r, errObjectIdTooLarge
		}
		cr.ObjectId = string(buf[:idx])
		sha.Write(buf[idx:])
		if _, err = io.CopyN(sha, f, n-100); err != nil {
			return r, errors.Tag(err, "sha")
		}
		buf = sha.Sum(buf[:0])
		h := sharedTypes.Hash(base64.RawURLEncoding.EncodeToString(buf))
		_, exists := t.WrittenHashes[h]
		if exists {
			t.WrittenHashes[h] = 0
			cr.Hash = h
			r = append(r, cr)
			continue
		}
		if _, err = f.Seek(int64(cr.Start), io.SeekStart); err != nil {
			return r, errors.Tag(err, "seek 2")
		}
		err = copyFile.AtomicN(t.contentDir.Join(h), f, publicRead, n-idx)
		if err != nil {
			return r, errors.Tag(err, "copy sha")
		}
		t.WrittenHashes[h] = 0
		cr.Hash = h
		r = append(r, cr)
	}
	return r, nil
}

var (
	errNotAFile         = errors.New("output.pdfxref is not a file")
	errTooLarge         = errors.New("output.pdfxref is too large")
	errObjectIdTooLarge = errors.New("objectId is too large")

	uncompressed = []byte("/0: uncompressed; offset = ")
	objToken     = []byte("obj")
)

const (
	minChunkSize   = 1024
	publicRead     = 0o644
	stageAge       = 5
	maxPDFXrefSize = 1024 * 1024

	pdfName     = sharedTypes.PathName("output.pdf")
	pdfXrefName = sharedTypes.PathName(constants.PDFCachingXrefFilename)
)

func parseXref(p string) ([]types.PDFCachingRange, error) {
	f, err2 := os.OpenFile(p, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err2 != nil {
		return nil, errors.Tag(err2, "open")
	}
	defer func() { _ = f.Close() }()
	if s, err := f.Stat(); err != nil {
		return nil, errors.Tag(err, "stat")
	} else if !s.Mode().IsRegular() {
		return nil, errNotAFile
	} else if s.Size() > maxPDFXrefSize {
		return nil, errTooLarge
	} else if s.Size() == 0 {
		return nil, nil
	}
	s := bufio.NewScanner(f)
	var r []types.PDFCachingRange
	for s.Scan() {
		b := s.Bytes()
		if len(b) < len(uncompressed) {
			continue
		}
		idx := bytes.IndexByte(b[:10], '/')
		if idx < 1 || !bytes.HasPrefix(b[idx:], uncompressed) {
			continue
		}
		n, err := strconv.ParseUint(string(b[idx+len(uncompressed):]), 10, 64)
		if err != nil {
			return nil, err
		}
		r = append(r, types.PDFCachingRange{
			Start: n,
		})
	}
	return r, s.Err()
}

type tracker struct {
	WrittenHashes map[sharedTypes.Hash]uint8
	LastRanges    []types.PDFCachingRange
	LastBuild     types.BuildId
	contentDir    types.ContentDir
	o             types.OutputDir
}

func (t *tracker) cleanupOld() {
	for hash, v := range t.WrittenHashes {
		if v < stageAge {
			t.WrittenHashes[hash] = v + 1
			continue
		}
		delete(t.WrittenHashes, hash)
		p := t.contentDir.Join(hash)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			log.Printf("cleanup pdfCaching failed for: %q: %s", p, err)
		}
	}
}

func (t *tracker) open() {
	if err := t.read(); err != nil {
		log.Printf("read pdfCaching tracker: %q: %s", t.contentDir, err)
	}
	if t.WrittenHashes == nil {
		t.WrittenHashes = make(map[sharedTypes.Hash]uint8)
	}
}

func (t *tracker) read() error {
	f, err := os.OpenFile(t.o.Tracker(), os.O_RDONLY, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }()
	return json.NewDecoder(f).Decode(&t)
}

func (t *tracker) flush() {
	t.cleanupOld()
	if err := t.write(); err != nil {
		log.Printf("persist pdfCaching tracker: %q: %s", t.contentDir, err)
	}
}

func (t *tracker) write() error {
	f, err := os.CreateTemp(string(t.o), "")
	if err != nil {
		return errors.Tag(err, "open")
	}
	err = json.NewEncoder(f).Encode(t)
	err2 := f.Close()
	if err != nil {
		return errors.Tag(err, "encode")
	}
	if err2 != nil {
		return errors.Tag(err2, "close")
	}
	if err = os.Rename(f.Name(), t.o.Tracker()); err != nil {
		return errors.Tag(err, "rename")
	}
	return nil
}
