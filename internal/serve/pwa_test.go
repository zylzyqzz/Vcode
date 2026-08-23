package serve

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vcode/internal/config"
	"vcode/internal/control"
)

// The PWA assets must be reachable before login: browsers fetch the manifest
// and icons without session context when deciding whether the site is
// installable, and the service worker must register from the login page too.
func TestPWAAvailableWithoutAuth(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{
		AuthMode:     "password",
		PasswordHash: "$2a$12$C6UzMDM.H6dfI/f/IKcEe.6uSMj4XCkL1RkEYbEyrxqXZrASKv7b6", // bcrypt("password")
	}).Handler())
	defer srv.Close()

	for path, wantCT := range map[string]string{
		"/manifest.webmanifest": "application/manifest+json",
		"/sw.js":                "text/javascript",
		"/icons/icon-192.png":   "image/png",
		"/icons/icon-512.png":   "image/png",
		"/apple-touch-icon.png": "image/png",
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status %d, want 200", path, resp.StatusCode)
			continue
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, wantCT) {
			t.Errorf("GET %s: Content-Type %q, want prefix %q", path, ct, wantCT)
		}
		if len(body) == 0 {
			t.Errorf("GET %s: empty body", path)
		}
	}

	// Unknown icon names must 404 (the handler serves from a fixed allowlist).
	resp, err := http.Get(srv.URL + "/icons/nope.png")
	if err != nil {
		t.Fatalf("GET unknown icon: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET unknown icon: status %d, want 404", resp.StatusCode)
	}

	// Path traversal must never be served as a public asset: the Go http
	// client normalizes dot segments before sending, so this arrives as a
	// non-public path and the auth gate rejects it (401 in password mode).
	resp, err = http.Get(srv.URL + "/icons/../../go.mod")
	if err != nil {
		t.Fatalf("GET traversal: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Errorf("GET traversal icon: status 200, want auth rejection")
	}
}

func TestServiceWorkerHeaders(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/sw.js")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Service-Worker-Allowed"); got != "/" {
		t.Errorf("Service-Worker-Allowed = %q, want \"/\"", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("sw.js Cache-Control = %q, want no-cache", got)
	}
}

// The shell must advertise the manifest so browsers offer installation.
func TestIndexLinksManifest(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	for _, frag := range []string{`rel="manifest"`, `navigator.serviceWorker.register`} {
		if !strings.Contains(string(body), frag) {
			t.Errorf("index.html missing %q", frag)
		}
	}
}
