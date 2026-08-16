# go-uaedr

A small Go client for Ukraine's **ЄДР** (Unified State Register of Legal Entities) open-data
export, published by the Ministry of Justice at [data.gov.ua](https://data.gov.ua) (dataset
`03cc1239-3988-4451-aa0d-aadb77448714`, resource `uo.zip`), updated weekly, free.

This is a register of **all** legal entities, not just religious organizations — `Reader.Next`
streams every `<SUBJECT>` element; use `Subject.IsReligiousOrg()` to filter to OPF code 825
(КОПФГ "Релігійна організація") if that's what you're after.

## Install

```sh
go get github.com/olehmushka/go-uaedr
```

## Usage

Local file (`uo.xml` or a single-entry `uo.zip`):

```go
r, closer, err := uaedr.OpenFile("uo.zip")
if err != nil {
    // ...
}
defer closer.Close()

for {
    s, err := r.Next()
    if err == io.EOF {
        break
    }
    if err != nil {
        // ...
    }
    if s.IsReligiousOrg() {
        fmt.Println(s.EDRPOU, s.Name)
    }
}
```

Live HTTP source (streamed — never buffers the whole export, which can run several hundred MB):

```go
r, closer, err := uaedr.OpenHTTP(ctx, "https://data.gov.ua/.../uo.zip", uaedr.OpenHTTPOptions{})
```

`Reader` is a **plain forward-only iterator** — it doesn't batch, track a resume cursor, or guard
against concurrent use. If your application needs any of that (e.g. resumable paging through a
long-running job), build it on top of one `Reader` per run; this package intentionally doesn't own
that decision, since it depends entirely on how you're consuming the export.

## Notes from real-world use

- **Schema, verified against the dataset's own published `uo_schema.zip` XSD, not inferred**: a
  flat stream of `<SUBJECT>` elements. `Subject` models `NAME`, `SHORT_NAME`, `OPF`
  (organizational-legal form), `EDRPOU` (the unique registration code), and `STAN` (activity
  status) — the fields needed to identify and name an entity, not the full legal record.
- **OPF matching must be case-insensitive.** The classifier's own title-case text is
  "Релігійна організація", but the real, live export stores it **all uppercase**
  ("РЕЛІГІЙНА ОРГАНІЗАЦІЯ") — an exact match finds zero real rows. `Subject.IsReligiousOrg()`
  handles this for you.
- **No address field at all.** An older, superseded schema had one; the current export does not.
- **Encoding is windows-1251**, verified directly against the downloaded export's own XML prolog
  (`encoding="windows-1251"`) — the schema XSD's own "UTF-8" declaration describes the XSD file
  itself, not the data file. `Reader` auto-detects from the prolog, so this is handled for you
  either way.
- **STAN (activity status) is not filtered here.** The exact values distinguishing an active
  organization from a terminated one aren't yet characterized — every record is yielded regardless
  of STAN, with the raw value available on `Subject.Stan` for the caller to decide.

## License

Apache 2.0 — see [LICENSE](./LICENSE).
