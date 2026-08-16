// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package uaedr

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenHTTPOptions configures OpenHTTP. The zero value is valid — UserAgent defaults to
// DefaultUserAgent, HTTPClient defaults to an explicit no-Timeout *http.Client (this is a
// multi-hundred-MB streaming download; a fixed deadline would fail slow connections arbitrarily —
// ctx passed to OpenHTTP/Reader.Next is the only cancellation mechanism a caller needs).
type OpenHTTPOptions struct {
	HTTPClient *http.Client
	UserAgent  string
}

// OpenHTTP issues a GET against sourceURL (a live uo.zip/uo.xml export) and returns a Reader
// streaming directly from the response body — never buffers the whole response, since a real
// export can run several hundred MB. If sourceURL ends in .zip, the response is decompressed on
// the fly via NewStreamingZipEntryReader (archive/zip needs an io.ReaderAt / a fully-seekable
// source, which a streamed HTTP body isn't).
//
// The returned io.Closer must be called when done — it releases the underlying response body (and
// the zip decompressor, if applicable). Callers that need to hold the stream open across multiple
// Reader.Next calls spanning a long-running batch job (rather than draining it in one loop) own
// that lifecycle themselves; this function has no opinion on batching, resumable cursors, or
// concurrent-access guards — see this package's own doc comment.
func OpenHTTP(ctx context.Context, sourceURL string, opts OpenHTTPOptions) (*Reader, io.Closer, error) {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{}
	}
	userAgent := opts.UserAgent
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("uaedr: build request for %s: %w", sourceURL, err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("uaedr: GET %s: %w", sourceURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		_ = resp.Body.Close()
		return nil, nil, fmt.Errorf("uaedr: GET %s: unexpected status %s: %s", sourceURL, resp.Status, body)
	}

	if !strings.HasSuffix(strings.ToLower(sourceURL), ".zip") {
		return NewReader(resp.Body), resp.Body, nil
	}

	unzip, err := NewStreamingZipEntryReader(resp.Body)
	if err != nil {
		_ = resp.Body.Close()
		return nil, nil, err
	}
	return NewReader(unzip), closerFunc(func() error {
		unzipErr := unzip.Close()
		bodyErr := resp.Body.Close()
		if unzipErr != nil {
			return unzipErr
		}
		return bodyErr
	}), nil
}
