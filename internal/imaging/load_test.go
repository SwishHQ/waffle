package imaging

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pngHandler(body []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	})
}

func TestSchemeOf(t *testing.T) {
	cases := []struct {
		src, want string
	}{
		{"http://example.com/a.png", "http"},
		{"HTTPS://example.com/a.png", "https"},
		{"data:image/png;base64,AAAA", "data"},
		{"ftp://example.com/a.png", "ftp"},
		{"file:///tmp/a.png", "file"},
		{"custom+x://y", "custom+x"},
		{`C:\images\a.png`, ""}, // Windows drive letter, not a scheme
		{"./a.png", ""},
		{"images/a.png", ""},
		{"/abs/a.png", ""},
		{"a.png", ""},
		{":weird", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := schemeOf(c.src); got != c.want {
			t.Errorf("schemeOf(%q) = %q, want %q", c.src, got, c.want)
		}
	}
}

func TestLoadDataURI(t *testing.T) {
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(makePNG(t, 2, 2, false))
	im, err := Load(uri)
	if err != nil {
		t.Fatal(err)
	}
	if im.Width != 2 || im.Height != 2 {
		t.Errorf("dims = %dx%d, want 2x2", im.Width, im.Height)
	}
}

func TestLoadHTTP(t *testing.T) {
	srv := httptest.NewServer(pngHandler(makePNG(t, 4, 3, false)))
	defer srv.Close()

	im, err := Load(srv.URL + "/img.png")
	if err != nil {
		t.Fatal(err)
	}
	if im.Width != 4 || im.Height != 3 || im.Format != "png" {
		t.Errorf("image = %dx%d %s, want 4x3 png", im.Width, im.Height, im.Format)
	}

	// Scheme matching is case-insensitive.
	if _, err := Load("HTTP" + srv.URL[len("http"):] + "/img.png"); err != nil {
		t.Errorf("uppercase scheme: %v", err)
	}
}

func TestLoadHTTPS(t *testing.T) {
	srv := httptest.NewTLSServer(pngHandler(makePNG(t, 5, 5, false)))
	defer srv.Close()

	old := httpClient
	httpClient = srv.Client() // trusts the test server's certificate
	defer func() { httpClient = old }()

	im, err := Load(srv.URL + "/img.png")
	if err != nil {
		t.Fatal(err)
	}
	if im.Width != 5 || im.Height != 5 {
		t.Errorf("dims = %dx%d, want 5x5", im.Width, im.Height)
	}
}

func TestLoadURLStatusError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	_, err := Load(srv.URL + "/missing.png")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("want 404 status error, got %v", err)
	}
}

func TestLoadURLNotAnImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>not an image</html>"))
	}))
	defer srv.Close()

	_, err := Load(srv.URL + "/page.html")
	if err == nil || !strings.Contains(err.Error(), "unrecognized image format") {
		t.Errorf("want decode error, got %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), srv.URL) {
		t.Errorf("error should mention the source URL, got %v", err)
	}
}

func TestLoadURLTooLarge(t *testing.T) {
	srv := httptest.NewServer(pngHandler(makePNG(t, 8, 8, false)))
	defer srv.Close()

	old := maxRemoteImageBytes
	maxRemoteImageBytes = 16
	defer func() { maxRemoteImageBytes = old }()

	_, err := Load(srv.URL + "/big.png")
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("want size-limit error, got %v", err)
	}
}

func TestLoadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "img.png")
	if err := os.WriteFile(path, makePNG(t, 3, 2, false), 0o644); err != nil {
		t.Fatal(err)
	}
	im, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if im.Width != 3 || im.Height != 2 {
		t.Errorf("dims = %dx%d, want 3x2", im.Width, im.Height)
	}
}

func TestLoadFileMissing(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.png"))
	if err == nil || !strings.Contains(err.Error(), "imaging:") {
		t.Errorf("want wrapped read error, got %v", err)
	}
}

func TestLoadFileNotAnImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("plain text"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("decode error should mention the file path, got %v", err)
	}
}

func TestLoadUnsupportedScheme(t *testing.T) {
	for _, src := range []string{
		"ftp://example.com/a.png",
		"file:///tmp/a.png",
		"gopher://example.com/a.png",
	} {
		_, err := Load(src)
		if err == nil || !strings.Contains(err.Error(), "unsupported scheme") {
			t.Errorf("Load(%q): want unsupported-scheme error, got %v", src, err)
			continue
		}
		scheme := src[:strings.Index(src, ":")]
		if !strings.Contains(err.Error(), scheme) {
			t.Errorf("Load(%q): error should name the scheme %q, got %v", src, scheme, err)
		}
	}
}

func TestLoadWindowsDrivePathIsFile(t *testing.T) {
	_, err := Load(`C:\images\logo.png`)
	if err == nil {
		t.Skip("path unexpectedly exists")
	}
	if strings.Contains(err.Error(), "unsupported scheme") {
		t.Errorf("drive-letter path treated as scheme: %v", err)
	}
}
