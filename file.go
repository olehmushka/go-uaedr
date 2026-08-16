// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package uaedr

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"strings"
)

// OpenFile opens a local uo.xml or single-entry uo.zip export (the real shape data.gov.ua
// publishes) and returns a Reader positioned at the start, plus an io.Closer that must be called
// when done. Zip mode uses the stdlib archive/zip against the file's own ReaderAt, since a local
// file supports seeking — unlike OpenHTTP, which streams and cannot use archive/zip for that
// reason (see NewStreamingZipEntryReader).
func OpenFile(path string) (*Reader, io.Closer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("uaedr: open %s: %w", path, err)
	}
	if !strings.HasSuffix(strings.ToLower(path), ".zip") {
		return NewReader(f), f, nil
	}

	// f itself must stay open for the whole streaming read below — zip.File.Open()'s returned
	// reader reads lazily from f's underlying ReaderAt, not eagerly into memory, so closing f here
	// would break every read after this function returns.
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, fmt.Errorf("uaedr: stat %s: %w", path, err)
	}
	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		_ = f.Close()
		return nil, nil, fmt.Errorf("uaedr: open zip %s: %w", path, err)
	}
	for _, zf := range zr.File {
		if strings.HasSuffix(strings.ToLower(zf.Name), ".xml") {
			rc, err := zf.Open()
			if err != nil {
				_ = f.Close()
				return nil, nil, fmt.Errorf("uaedr: open %s in zip: %w", zf.Name, err)
			}
			return NewReader(rc), closerFunc(func() error {
				rcErr := rc.Close()
				fErr := f.Close()
				if rcErr != nil {
					return rcErr
				}
				return fErr
			}), nil
		}
	}
	_ = f.Close()
	return nil, nil, fmt.Errorf("uaedr: no .xml entry found in %s", path)
}
