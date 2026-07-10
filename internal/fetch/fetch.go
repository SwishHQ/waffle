// Package fetch resolves a resource reference — a data: URI, an http(s) URL, or
// a local file path — to raw bytes, with a size cap and a scheme allowlist. It
// is the shared byte loader for external assets such as registered fonts.
package fetch

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// httpClient performs remote fetches. It is a variable so tests can substitute a
// client (e.g. one that trusts a test TLS server).
var httpClient = &http.Client{Timeout: 30 * time.Second}

// maxBytes caps how much a single fetch reads. It is a variable so tests can
// lower it.
var maxBytes int64 = 64 << 20 // 64 MiB

// Bytes resolves src to raw bytes. Supported sources: data: URIs, http:// and
// https:// URLs, and local file paths (no scheme). Any other scheme is rejected
// with a descriptive error.
func Bytes(src string) ([]byte, error) {
	switch scheme := schemeOf(src); scheme {
	case "data":
		return dataURI(src)
	case "http", "https":
		return fetchURL(src)
	case "":
		return os.ReadFile(src)
	default:
		return nil, fmt.Errorf("fetch: unsupported scheme %q in %q (supported: http(s) URLs, data: URIs, file paths)", scheme, src)
	}
}

// fetchURL retrieves an http(s) resource, enforcing the byte cap.
func fetchURL(url string) ([]byte, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	if int64(len(raw)) > maxBytes {
		return nil, fmt.Errorf("fetch %s: response exceeds %d-byte limit", url, maxBytes)
	}
	return raw, nil
}

// dataURI decodes the payload of a data: URI (base64 or raw percent-free text).
func dataURI(uri string) ([]byte, error) {
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		return nil, fmt.Errorf("fetch: malformed data URI")
	}
	meta, payload := uri[:comma], uri[comma+1:]
	if strings.Contains(meta, ";base64") {
		return base64.StdEncoding.DecodeString(payload)
	}
	return []byte(payload), nil
}

// schemeOf returns src's lowercased URI scheme ("http", "data", …), or "" if src
// has none and should be treated as a file path. Single-letter schemes are
// treated as Windows drive letters (e.g. `C:\fonts\Inter.ttf`), not schemes.
func schemeOf(src string) string {
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
			// Valid scheme character.
		case i > 0 && ('0' <= c && c <= '9' || c == '+' || c == '-' || c == '.'):
			// Valid after the first character.
		case c == ':' && i >= 2:
			return strings.ToLower(src[:i])
		default:
			return ""
		}
	}
	return ""
}
