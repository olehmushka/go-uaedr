// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package uaedr

import (
	"archive/zip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOpenHTTPEndToEnd serves a small crafted zip over httptest and drives OpenHTTP through a full
// read, proving the streaming unzip + XML decode chain works end-to-end, not just each piece in
// isolation.
func TestOpenHTTPEndToEnd(t *testing.T) {
	xmlBody := `<?xml version="1.0" encoding="utf-8"?><SUBJECTS>` +
		`<SUBJECT><RECORD>1</RECORD><NAME>Church A</NAME><OPF>РЕЛІГІЙНА ОРГАНІЗАЦІЯ</OPF><EDRPOU>111</EDRPOU></SUBJECT>` +
		`<SUBJECT><RECORD>2</RECORD><NAME>Some LLC</NAME><OPF>ТОВАРИСТВО</OPF><EDRPOU>222</EDRPOU></SUBJECT>` +
		`<SUBJECT><RECORD>3</RECORD><NAME>Church B</NAME><OPF>релігійна організація</OPF><EDRPOU>333</EDRPOU></SUBJECT>` +
		`</SUBJECTS>`
	zb := buildTestZip(t, "uo.xml", zip.Deflate, []byte(xmlBody))

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zb)
	}))
	defer srv.Close()

	r, closer, err := OpenHTTP(context.Background(), srv.URL+"/uo.zip", OpenHTTPOptions{})
	if err != nil {
		t.Fatalf("OpenHTTP: %v", err)
	}
	defer func() { _ = closer.Close() }()

	if gotUA != DefaultUserAgent {
		t.Fatalf("User-Agent = %q, want default %q", gotUA, DefaultUserAgent)
	}

	var got []string
	for {
		s, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if s.IsReligiousOrg() {
			got = append(got, s.EDRPOU)
		}
	}

	if len(got) != 2 || got[0] != "111" || got[1] != "333" {
		t.Fatalf("got %v, want [111 333] (the LLC record must be filtered out by IsReligiousOrg)", got)
	}
}

func TestOpenHTTPCustomUserAgent(t *testing.T) {
	xmlBody := `<?xml version="1.0" encoding="utf-8"?><SUBJECTS></SUBJECTS>`

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(xmlBody))
	}))
	defer srv.Close()

	_, closer, err := OpenHTTP(context.Background(), srv.URL+"/uo.xml", OpenHTTPOptions{UserAgent: "myapp/1.0"})
	if err != nil {
		t.Fatalf("OpenHTTP: %v", err)
	}
	defer func() { _ = closer.Close() }()

	if gotUA != "myapp/1.0" {
		t.Fatalf("User-Agent = %q, want the overridden value", gotUA)
	}
}

func TestOpenHTTPUpstreamFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if _, _, err := OpenHTTP(context.Background(), srv.URL+"/uo.zip", OpenHTTPOptions{}); err == nil {
		t.Fatal("a real 404 from the source should be a real error")
	}
}

func TestOpenHTTPPlainXML(t *testing.T) {
	xmlBody := `<?xml version="1.0" encoding="utf-8"?><SUBJECTS>` +
		`<SUBJECT><NAME>Church</NAME><OPF>Релігійна організація</OPF><EDRPOU>1</EDRPOU></SUBJECT>` +
		`</SUBJECTS>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xmlBody))
	}))
	defer srv.Close()

	r, closer, err := OpenHTTP(context.Background(), srv.URL+"/uo.xml", OpenHTTPOptions{})
	if err != nil {
		t.Fatalf("OpenHTTP: %v", err)
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
