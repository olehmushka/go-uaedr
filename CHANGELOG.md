# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-08-16

### Added

- Initial extraction from open-faith-map: `Reader`/`OpenFile`/`OpenHTTP` for Ukraine's ЄДР
  (Unified State Register of Legal Entities) XML/zip export, including windows-1251 charset
  handling and a hand-rolled streaming zip-entry reader for the HTTP-streaming path.

[Unreleased]: https://github.com/olehmushka/go-uaedr/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/olehmushka/go-uaedr/releases/tag/v0.1.0
