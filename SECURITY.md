# Security Policy

## Supported versions

Only the latest tagged release is supported. Please upgrade before reporting an issue that may
already be fixed.

## Reporting a vulnerability

Please do **not** open a public GitHub issue for security vulnerabilities.

- **Preferred**: use GitHub's private vulnerability reporting for this repository
  (the "Security" tab → "Report a vulnerability").
- **Alternative**: email olegamysk@gmail.com with details and, if possible, a proof of concept.

You should get an initial response within a few days.

## Scope notes

This package makes outbound HTTP requests to a caller-supplied `sourceURL` (the real Ukrainian
government export by default), decodes XML, and — for `.zip` sources — includes a hand-rolled
streaming zip-entry reader (`zip.go`) that parses a local file header directly rather than using
`archive/zip`. That hand-rolled parser is the most security-sensitive code in this repository: it
reads attacker-influenced length fields from the zip header before allocating/streaming — if you
find an input that causes excessive memory use, a panic, or a hang there, please report it via the
channels above rather than a public issue.
