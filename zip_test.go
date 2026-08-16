// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package uaedr

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildTestZip returns a single-entry zip (name/method as given) containing content — real bytes
// via archive/zip.Writer, not hand-rolled, so the streaming reader is tested against a genuinely
// valid zip, not a reader's own idea of one.
func buildTestZip(t *testing.T, name string, method uint16, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
	if err != nil {
		t.Fatalf("CreateHeader: %v", err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip Close: %v", err)
	}
	return buf.Bytes()
}

func TestNewStreamingZipEntryReaderDeflate(t *testing.T) {
	want := []byte(strings.Repeat("<SUBJECT><NAME>test</NAME></SUBJECT>", 1000)) // compressible
	zb := buildTestZip(t, "uo.xml", zip.Deflate, want)

	rc, err := NewStreamingZipEntryReader(bytes.NewReader(zb))
	if err != nil {
		t.Fatalf("NewStreamingZipEntryReader: %v", err)
	}
	defer func() { _ = rc.Close() }()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read decompressed: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("decompressed content mismatch: got %d bytes, want %d bytes", len(got), len(want))
	}
}

// buildRawStoreLocalFileHeader hand-builds a zip local file header with the STORE method and the
// data-descriptor bit (general-purpose flag bit 3) explicitly clear, i.e. sizes declared up front
// in the header itself — the one shape NewStreamingZipEntryReader can stream STORE from without a
// central directory. archive/zip.Writer cannot produce this: empirically (verified against the Go
// toolchain), it always sets the data-descriptor bit for a streamed Write, even against a seekable
// *os.File target, so this case is built by hand against the exact fixed-offset layout
// NewStreamingZipEntryReader itself parses.
func buildRawStoreLocalFileHeader(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString("PK\x03\x04")
	writeUint16LE(&buf, 20)                   // version needed — arbitrary, unread by the parser
	writeUint16LE(&buf, 0)                    // general-purpose flag — bit 3 (data descriptor) clear
	writeUint16LE(&buf, zip.Store)            // compression method
	writeUint16LE(&buf, 0)                    // last mod time — arbitrary
	writeUint16LE(&buf, 0)                    // last mod date — arbitrary
	writeUint32LE(&buf, 0)                    // CRC-32 — unread by the parser
	writeUint32LE(&buf, uint32(len(content))) // compressed size == uncompressed size for STORE
	writeUint32LE(&buf, uint32(len(content))) // uncompressed size
	writeUint16LE(&buf, uint16(len(name)))    // filename length
	writeUint16LE(&buf, 0)                    // extra field length
	buf.WriteString(name)
	buf.Write(content)
	return buf.Bytes()
}

func writeUint16LE(buf *bytes.Buffer, v uint16) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
}

func writeUint32LE(buf *bytes.Buffer, v uint32) {
	buf.WriteByte(byte(v))
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v >> 16))
	buf.WriteByte(byte(v >> 24))
}

func TestNewStreamingZipEntryReaderStore(t *testing.T) {
	want := []byte("<SUBJECT><NAME>stored, not deflated</NAME></SUBJECT>")
	zb := buildRawStoreLocalFileHeader(t, "uo.xml", want)

	rc, err := NewStreamingZipEntryReader(bytes.NewReader(zb))
	if err != nil {
		t.Fatalf("NewStreamingZipEntryReader: %v", err)
	}
	defer func() { _ = rc.Close() }()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read stored content: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("stored content mismatch: got %q, want %q", got, want)
	}
}

// TestNewStreamingZipEntryReaderStoreWithDataDescriptorRejected confirms the one documented gap in
// NewStreamingZipEntryReader's own doc comment: STORE with the data-descriptor bit set (unknown
// length up front) is refused with a clear error rather than silently reading garbage or hanging.
// This is also, empirically, what Go's own archive/zip.Writer produces for a streamed STORE entry
// (verified via buildTestZip above) — not just a hypothetical shape.
func TestNewStreamingZipEntryReaderStoreWithDataDescriptorRejected(t *testing.T) {
	zb := buildTestZip(t, "uo.xml", zip.Store, []byte("some content"))
	_, err := NewStreamingZipEntryReader(bytes.NewReader(zb))
	if err == nil {
		t.Fatal("expected an error for STORE with the data-descriptor bit set, got nil")
	}
}

func TestNewStreamingZipEntryReaderBadSignature(t *testing.T) {
	_, err := NewStreamingZipEntryReader(bytes.NewReader(bytes.Repeat([]byte{0x00}, 30)))
	if err == nil {
		t.Fatal("expected an error for a non-zip byte stream, got nil")
	}
}

func TestNewStreamingZipEntryReaderTruncated(t *testing.T) {
	_, err := NewStreamingZipEntryReader(bytes.NewReader([]byte("PK\x03\x04short")))
	if err == nil {
		t.Fatal("expected an error for a truncated local file header, got nil")
	}
}

func TestOpenFileZip(t *testing.T) {
	xmlBody := `<?xml version="1.0" encoding="utf-8"?><SUBJECTS>` +
		`<SUBJECT><NAME>Church</NAME><OPF>Релігійна організація</OPF><EDRPOU>1</EDRPOU></SUBJECT>` +
		`</SUBJECTS>`
	zb := buildTestZip(t, "uo.xml", zip.Deflate, []byte(xmlBody))

	path := filepath.Join(t.TempDir(), "uo.zip")
	if err := os.WriteFile(path, zb, 0o600); err != nil {
		t.Fatal(err)
	}

	r, closer, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer func() { _ = closer.Close() }()

	s, err := r.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if s.EDRPOU != "1" {
		t.Fatalf("EDRPOU = %q, want \"1\"", s.EDRPOU)
	}
}
