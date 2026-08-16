// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package uaedr

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestReaderNextReturnsEverySubjectUnfiltered(t *testing.T) {
	xmlBody := `<?xml version="1.0" encoding="utf-8"?><SUBJECTS>` +
		`<SUBJECT><RECORD>1</RECORD><NAME>Church A</NAME><OPF>РЕЛІГІЙНА ОРГАНІЗАЦІЯ</OPF><EDRPOU>111</EDRPOU></SUBJECT>` +
		`<SUBJECT><RECORD>2</RECORD><NAME>Some LLC</NAME><OPF>ТОВАРИСТВО</OPF><EDRPOU>222</EDRPOU></SUBJECT>` +
		`<SUBJECT><RECORD>3</RECORD><NAME>Church B</NAME><OPF>релігійна організація</OPF><EDRPOU>333</EDRPOU></SUBJECT>` +
		`</SUBJECTS>`

	r := NewReader(strings.NewReader(xmlBody))
	var got []Subject
	for {
		s, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		got = append(got, s)
	}

	if len(got) != 3 {
		t.Fatalf("got %d subjects, want 3 (Next must not filter by OPF)", len(got))
	}
	if got[1].EDRPOU != "222" || got[1].IsReligiousOrg() {
		t.Fatalf("subject 2 (a plain LLC) should not report IsReligiousOrg()==true: %+v", got[1])
	}
}

// TestIsReligiousOrgCaseInsensitive is the regression test for this package's own real, live
// finding: the export stores OPF as all-uppercase "РЕЛІГІЙНА ОРГАНІЗАЦІЯ", not the classifier's own
// title-case text — an exact match against ReligiousOrgOPF would match zero real rows.
func TestIsReligiousOrgCaseInsensitive(t *testing.T) {
	cases := []struct {
		opf  string
		want bool
	}{
		{"Релігійна організація", true},
		{"РЕЛІГІЙНА ОРГАНІЗАЦІЯ", true},
		{"релігійна організація", true},
		{"  Релігійна організація  ", true}, // real export rows have been seen with stray whitespace
		{"ТОВАРИСТВО", false},
		{"", false},
	}
	for _, c := range cases {
		got := Subject{OPF: c.opf}.IsReligiousOrg()
		if got != c.want {
			t.Errorf("IsReligiousOrg(OPF=%q) = %v, want %v", c.opf, got, c.want)
		}
	}
}

// TestCharsetWindows1251 confirms Reader correctly decodes this source's real, verified encoding —
// a windows-1251-encoded document declared as such in its own XML prolog, not UTF-8.
func TestCharsetWindows1251(t *testing.T) {
	const wantName = "Церква Христа" // real Cyrillic text, round-tripped through cp1251 below
	utf8Body := `<?xml version="1.0" encoding="windows-1251"?><SUBJECTS><SUBJECT><NAME>` +
		wantName + `</NAME><OPF>Релігійна організація</OPF></SUBJECT></SUBJECTS>`

	var encoded bytes.Buffer
	w := charmap.Windows1251.NewEncoder().Writer(&encoded)
	if _, err := io.WriteString(w, utf8Body); err != nil {
		t.Fatalf("encode fixture to windows-1251: %v", err)
	}

	r := NewReader(bytes.NewReader(encoded.Bytes()))
	s, err := r.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if s.Name != wantName {
		t.Fatalf("Name = %q, want %q — windows-1251 decoding broke", s.Name, wantName)
	}
}

func TestOpenFileXML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "uo.xml")
	body := `<?xml version="1.0" encoding="utf-8"?><SUBJECTS>` +
		`<SUBJECT><NAME>Church</NAME><OPF>Релігійна організація</OPF><EDRPOU>1</EDRPOU></SUBJECT>` +
		`</SUBJECTS>`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
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
	if _, err := r.Next(); err != io.EOF {
		t.Fatalf("second Next: got err=%v, want io.EOF", err)
	}
}

func TestOpenFileMultiRecord(t *testing.T) {
	const wantTotal = 1200

	dir := t.TempDir()
	path := filepath.Join(dir, "uo.xml")
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?><SUBJECTS>`)
	for i := 0; i < wantTotal; i++ {
		fmt.Fprintf(&sb, `<SUBJECT><NAME>Church %d</NAME><OPF>РЕЛІГІЙНА ОРГАНІЗАЦІЯ</OPF><EDRPOU>%d</EDRPOU></SUBJECT>`, i, i)
	}
	sb.WriteString(`</SUBJECTS>`)
	if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	r, closer, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer func() { _ = closer.Close() }()

	seen := 0
	for {
		_, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next (record %d): %v", seen, err)
		}
		seen++
	}
	if seen != wantTotal {
		t.Fatalf("got %d records, want %d", seen, wantTotal)
	}
}

func TestOpenFileMissing(t *testing.T) {
	if _, _, err := OpenFile(filepath.Join(t.TempDir(), "does-not-exist.xml")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}
