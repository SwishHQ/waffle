package fetch

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBytesDataURI(t *testing.T) {
	b, err := Bytes("data:font/ttf;base64," + base64.StdEncoding.EncodeToString([]byte("hello")))
	if err != nil || string(b) != "hello" {
		t.Fatalf("data URI = %q, %v; want \"hello\"", b, err)
	}
}

func TestBytesFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f.bin")
	if err := os.WriteFile(p, []byte("filedata"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Bytes(p)
	if err != nil || string(b) != "filedata" {
		t.Fatalf("file = %q, %v; want \"filedata\"", b, err)
	}
}

func TestBytesURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("remote"))
	}))
	defer srv.Close()
	b, err := Bytes(srv.URL + "/font.ttf")
	if err != nil || string(b) != "remote" {
		t.Fatalf("url = %q, %v; want \"remote\"", b, err)
	}
}

func TestBytesSchemeRejected(t *testing.T) {
	if _, err := Bytes("ftp://example.com/x.ttf"); err == nil {
		t.Error("ftp scheme should be rejected")
	}
}

func TestBytesURLCap(t *testing.T) {
	old := maxBytes
	maxBytes = 4
	defer func() { maxBytes = old }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", 100)))
	}))
	defer srv.Close()
	if _, err := Bytes(srv.URL); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("oversized response should be rejected with a limit error, got %v", err)
	}
}

func TestBytesHTTPStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()
	if _, err := Bytes(srv.URL); err == nil {
		t.Error("404 should be an error")
	}
}
