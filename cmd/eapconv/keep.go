package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/arloliu/go-secs/tracepack"
)

// errPackConflict reports a pack whose key already holds different bytes.
var errPackConflict = errors.New("a pack with the same key holds different bytes")

// keepSink is a SegmentSink that never replaces a committed segment (tracepack decision G5-192).
// A DirSink's Commit renames over an existing key,
// so keepSink compares a segment with the file already at its key before committing it:
// the same bytes leave that file as it is and discard the new one; different bytes fail with errPackConflict.
// The check and the rename are not atomic, so it holds for one converter writing a root at a time.
type keepSink struct {
	inner tracepack.SegmentSink
	root  string // the directory inner stores its keys under, with no prefix
}

// keepFile is a segment of a keepSink; it hashes what is written to it,
// which is the whole file, since a SegmentFile is written only in sequence.
type keepFile struct {
	tracepack.SegmentFile
	key    string
	path   string
	h      hash.Hash
	reused bool // Commit found the same bytes at key and kept them
}

var (
	_ tracepack.SegmentSink = (*keepSink)(nil)
	_ tracepack.SegmentFile = (*keepFile)(nil)
)

// fileSHA256 returns the SHA-256 of the file at path.
func fileSHA256(path string) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // a segment key under the operator's output directory
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

// Create creates the segment in the inner sink.
func (s *keepSink) Create(ctx context.Context, info tracepack.SegmentInfo) (tracepack.SegmentFile, error) {
	key, err := tracepack.SegmentKey("", info.ToolID, info.CaptureID, info.SeqFirst, info.PackID)
	if err != nil {
		return nil, err
	}
	f, err := s.inner.Create(ctx, info)
	if err != nil {
		return nil, err
	}

	return &keepFile{SegmentFile: f, key: key, path: filepath.Join(s.root, filepath.FromSlash(key)), h: sha256.New()}, nil
}

// Write writes p to the segment and to its hash.
func (f *keepFile) Write(p []byte) (int, error) {
	n, err := f.SegmentFile.Write(p)
	f.h.Write(p[:n])

	return n, err
}

// Commit commits the segment when its key is free,
// discards it when the key holds the same bytes, and fails with errPackConflict otherwise.
func (f *keepFile) Commit(ctx context.Context) error {
	old, err := fileSHA256(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return f.SegmentFile.Commit(ctx)
	}
	if abortErr := f.Abort(); err == nil {
		err = abortErr
	}
	if err != nil {
		return err
	}
	if !bytes.Equal(old, f.h.Sum(nil)) {
		return fmt.Errorf("%w: %s", errPackConflict, f.key)
	}
	f.reused = true

	return nil
}
