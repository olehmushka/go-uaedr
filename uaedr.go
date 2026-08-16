// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package uaedr is a small client for Ukraine's ЄДР (Unified State Register of Legal Entities)
// open-data export. Real, verified source: the Ministry of Justice publishes this as genuine open
// data at data.gov.ua (dataset id 03cc1239-3988-4451-aa0d-aadb77448714, resource "uo.zip" — legal
// entities), updated weekly, free.
//
// Real, verified schema (checked against the dataset's own published uo_schema.zip, an XSD, not
// inferred): the export is a flat stream of <SUBJECT> elements, each carrying (among many fields
// this package ignores) NAME, SHORT_NAME, OPF (organizational-legal form — "Організаційно-правова
// форма"), EDRPOU (the unique registration code), and STAN (activity status). Religious
// organizations are identified by OPF = ReligiousOrgOPF (КОПФГ classifier code 825, verified against
// data.gov.ua's own kopfg.json resource) — matched case-insensitively via Subject.IsReligiousOrg, a
// real correction found by scanning the actual downloaded export: the live data stores OPF as
// "РЕЛІГІЙНА ОРГАНІЗАЦІЯ" (all uppercase), not the classifier's own title-case text. An exact match
// would have matched zero real rows.
//
// Real, verified constraint: this export has NO address field at all (an older, now-superseded
// schema had one; the current one does not).
//
// Real, verified encoding: windows-1251 (checked directly against the actual downloaded export's
// XML prolog — `<?xml version="1.0" encoding="windows-1251"?>` — not assumed; the schema XSD's own
// "UTF-8" declaration describes the XSD file itself, not the data file). Reader's charsetReader
// auto-detects from the prolog rather than hardcoding either encoding.
//
// Reader is a plain forward-only iterator — it does not filter by OPF, does not track a resume
// cursor, and does not hold any run-scoped lock. A caller that needs batching, resumable cursors,
// or protection against two goroutines sharing one Reader owns that itself; every one of those
// concerns is about how the CALLER wants to consume a register export, not about the register's own
// wire format, so this package deliberately doesn't own any of it.
package uaedr

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// Code is a stable identifier for this source, useful for callers that track records by source.
const Code = "ua-edr"

// ReligiousOrgOPF is КОПФГ code 825's literal text — verified directly against data.gov.ua's own
// kopfg.json classifier resource, not guessed. The real data stores it uppercase; compare via
// Subject.IsReligiousOrg, not with ==.
const ReligiousOrgOPF = "Релігійна організація"

// DefaultUserAgent identifies this client. Callers embedding this package in their own product
// should override it via OpenHTTPOptions.UserAgent to identify their own application instead.
const DefaultUserAgent = "go-uaedr/0.1 (+https://github.com/olehmushka/go-uaedr)"

// Subject mirrors the real fields of a <SUBJECT> element this package models, per uo_schema.zip's
// published XSD. Every other real field (FOUNDERS, SIGNERS, BRANCHES, ...) is intentionally not
// modeled — this package only carries enough to identify and name a legal entity, not to
// reconstruct its full record.
type Subject struct {
	Record string `xml:"RECORD" json:"record"`
	Name   string `xml:"NAME" json:"name"`
	Short  string `xml:"SHORT_NAME" json:"shortName"`
	OPF    string `xml:"OPF" json:"opf"`
	EDRPOU string `xml:"EDRPOU" json:"edrpou"`
	Stan   string `xml:"STAN" json:"stan"`
}

// IsReligiousOrg reports whether s is registered under OPF code 825 (ReligiousOrgOPF), matched
// case-insensitively — see this package's own doc comment for why an exact match would miss every
// real row.
func (s Subject) IsReligiousOrg() bool {
	return strings.EqualFold(strings.TrimSpace(s.OPF), ReligiousOrgOPF)
}

// Reader streams <SUBJECT> elements from an already-open XML document, one at a time, forward only.
// The zero value is not usable — construct one with NewReader.
type Reader struct {
	dec *xml.Decoder
}

// NewReader wraps r in a Reader, wiring up charset auto-detection (see charsetReader) so a
// windows-1251-encoded document (this source's real encoding) decodes correctly without the caller
// having to know that up front.
func NewReader(r io.Reader) *Reader {
	dec := xml.NewDecoder(r)
	dec.CharsetReader = charsetReader
	return &Reader{dec: dec}
}

// Next returns the next <SUBJECT> element in document order, or io.EOF once the document is
// exhausted. It does not filter by OPF — callers that only want religious organizations should
// check Subject.IsReligiousOrg() themselves, since this export also carries every other legal
// entity type in the same stream.
func (r *Reader) Next() (Subject, error) {
	for {
		tok, err := r.dec.Token()
		if err != nil {
			return Subject{}, err // io.EOF propagates as-is
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "SUBJECT" {
			continue
		}
		var s Subject
		if err := r.dec.DecodeElement(&s, &start); err != nil {
			return Subject{}, fmt.Errorf("uaedr: decode SUBJECT: %w", err)
		}
		return s, nil
	}
}

// closerFunc adapts a plain func() error to io.Closer.
type closerFunc func() error

func (f closerFunc) Close() error { return f() }

// charsetReader lets xml.Decoder handle a non-UTF-8 prolog declaration (windows-1251, this source's
// real encoding) without failing outright — Go's stdlib xml package only understands UTF-8/US-ASCII
// natively.
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(charset) {
	case "utf-8", "us-ascii", "":
		return input, nil
	case "windows-1251", "cp1251":
		return charmap.Windows1251.NewDecoder().Reader(input), nil
	default:
		return nil, fmt.Errorf("uaedr: unsupported XML charset %q", charset)
	}
}
