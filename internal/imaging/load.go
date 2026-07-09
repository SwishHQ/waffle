package imaging

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// httpClient performs remote image fetches. It is a variable so tests can
// substitute a client (e.g. one that trusts a test TLS server).
var httpClient = &http.Client{Timeout: 30 * time.Second}

// maxRemoteImageBytes caps how much LoadURL reads from a response body. It is
// a variable so tests can lower it.
var maxRemoteImageBytes int64 = 64 << 20 // 64 MiB

// Load resolves an image source and decodes it. The source may be a data: URI,
// an http:// or https:// URL, or a local file path; any other URI scheme is
// rejected with a descriptive error.
func Load(src string) (*Image, error) {
	switch scheme := schemeOf(src); scheme {
	case "data":
		return DecodeDataURI(src)
	case "http", "https":
		return LoadURL(src)
	case "":
		return LoadFile(src)
	default:
		return nil, fmt.Errorf("imaging: unsupported scheme %q in image source %q (supported: http(s) URLs, data: URIs, and file paths)", scheme, src)
	}
}

// LoadURL fetches an image over HTTP(S) and decodes it.
func LoadURL(url string) (*Image, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("imaging: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("imaging: fetch %s: %s", url, resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("imaging: fetch %s: %w", url, err)
	}
	if int64(len(raw)) > maxRemoteImageBytes {
		return nil, fmt.Errorf("imaging: fetch %s: response exceeds %d-byte limit", url, maxRemoteImageBytes)
	}
	im, err := Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("%w (from %s)", err, url)
	}
	return im, nil
}

// LoadFile reads an image from the local filesystem and decodes it.
func LoadFile(path string) (*Image, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("imaging: %w", err)
	}
	im, err := Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("%w (file %s)", err, path)
	}
	return im, nil
}

// schemeOf returns src's lowercased URI scheme ("http", "data", …), or "" if
// src has none and should be treated as a file path. Single-letter schemes are
// treated as Windows drive letters (e.g. `C:\images\logo.png`), not schemes.
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
