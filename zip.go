// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package uaedr

import (
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// NewStreamingZipEntryReader parses ONE zip local file header by hand (signature "PK\x03\x04" at
// offset 0, general-purpose flag at offset 6, compression method at offset 8, filename/extra-field
// lengths at offsets 26/28 — the fixed 30-byte local file header layout) and hands the remaining
// stream straight to compress/flate (DEFLATE) or a length-bounded pass-through (STORE) — no
// archive/zip, no io.ReaderAt, true forward-only streaming: flate's own end-of-stream marker
// terminates decoding, independent of whatever size the zip metadata declared.
//
// This exists because archive/zip needs an io.ReaderAt (or the whole file) to read its central
// directory, which sits at the END of the file — unreachable without seeking or downloading
// everything first. A live HTTP response body offers neither, so OpenHTTP uses this instead for a
// .zip source.
//
// Assumes a single-entry zip, matching the real data.gov.ua uo.zip export (verified directly, not
// inferred) — this reads only the FIRST entry; it never reads the central directory. If the export
// is ever repacked multi-entry, this silently reads only that first entry — a documented
// assumption, not a silent one. STORE is only handled when its length is knowable up front (no
// data-descriptor bit set, nonzero declared size); DEFLATE has no such restriction, and DEFLATE is
// what the real export actually uses (its ~10x compression ratio rules out STORE).
func NewStreamingZipEntryReader(r io.Reader) (io.ReadCloser, error) {
	var header [30]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, fmt.Errorf("uaedr: read zip local file header: %w", err)
	}
	if string(header[0:4]) != "PK\x03\x04" {
		return nil, fmt.Errorf("uaedr: not a zip local file header (got % x)", header[0:4])
	}
	flags := binary.LittleEndian.Uint16(header[6:8])
	method := binary.LittleEndian.Uint16(header[8:10])
	compressedSize := binary.LittleEndian.Uint32(header[18:22])
	nameLen := binary.LittleEndian.Uint16(header[26:28])
	extraLen := binary.LittleEndian.Uint16(header[28:30])

	if _, err := io.CopyN(io.Discard, r, int64(nameLen)+int64(extraLen)); err != nil {
		return nil, fmt.Errorf("uaedr: skip zip filename/extra fields: %w", err)
	}

	const methodDeflate, methodStore = 8, 0
	switch method {
	case methodDeflate:
		return flate.NewReader(r), nil
	case methodStore:
		const dataDescriptorBit = 0x0008
		if flags&dataDescriptorBit != 0 || compressedSize == 0 {
			return nil, errors.New("uaedr: zip entry is STORE-compressed with an unknown streamed length (data-descriptor flag set) — not supported")
		}
		return io.NopCloser(io.LimitReader(r, int64(compressedSize))), nil
	default:
		return nil, fmt.Errorf("uaedr: unsupported zip compression method %d (only DEFLATE=8 and STORE=0 are handled)", method)
	}
}
