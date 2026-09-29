package ogmake

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// Shared vectors: packages/core/src/signing.test.ts, packages/laravel-ogmake/tests/SigningTest.php
const (
	secretB64     = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="
	canonicalWant = "format=png&template=basic&title=Hello%20World"
	sigWant       = "W8Xvrf9UTFKbVVQtofHq7A2ljK9z-nvJUL3B-rB7MY8"
	astralCanon   = "%F0%9F%98%80=astral&%EE%80%80=pua"
	astralSigWant = "Zr62H77WBS8WhCLT5Nt-vSrQ1Ldiot7_-W1XE335SPU"
)

func TestVector(t *testing.T) {
	c := CanonicalQuery(map[string]string{"template": "basic", "title": "Hello World", "format": "png", "sig": "x", "debug": "1"})
	if c != canonicalWant {
		t.Fatal(c)
	}
	secret, _ := DecodeSecret(secretB64)
	if s := SignQuery(secret, c); s != sigWant {
		t.Fatal(s)
	}
}

func TestAstralVector(t *testing.T) {
	c := CanonicalQuery(map[string]string{"\U0001F600": "astral", "": "pua"})
	if c != astralCanon {
		t.Fatal(c)
	}
	secret, _ := DecodeSecret(secretB64)
	if s := SignQuery(secret, c); s != astralSigWant {
		t.Fatal(s)
	}
}

func TestReservedAndBadSecret(t *testing.T) {
	if c := CanonicalQuery(map[string]string{"q": "a!b'c(d)e*f"}); c != "q=a%21b%27c%28d%29e%2Af" {
		t.Fatal(c)
	}
	if _, err := DecodeSecret("!!!"); err == nil {
		t.Fatal("want error")
	}
}

func TestURLs(t *testing.T) {
	u, err := SignedImageURL("", "key1", secretB64, "basic", map[string]string{"title": "Hello World"}, ImageOptions{Format: "png"})
	if err != nil || u != "https://ogmake.com/i/key1/"+sigWant+"?"+canonicalWant {
		t.Fatal(u, err)
	}
	if p := PublicImageURL("https://x.test/", "k", "a b"); p != "https://x.test/p/k/a%20b" {
		t.Fatal(p)
	}
}

func TestClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/images":
			if r.Header.Get("Authorization") != "Bearer og_live_x" {
				w.WriteHeader(401)
				w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			w.Write([]byte(`{"url":"https://ogmake.com/s/h.png","hash":"h","width":1200}`))
		case "/v1/usage":
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"invalid_input","detail":[{"field":"title","message":"required"}]}`))
		case "/v1/templates":
			http.Redirect(w, r, "https://evil.test", 302)
		default:
			w.WriteHeader(500)
			w.Write([]byte("<html>"))
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	c := &Client{APIKey: "og_live_x", BaseURL: srv.URL}
	res, err := c.Render(ctx, RenderRequest{Template: "basic", Params: map[string]interface{}{"title": "t"}})
	if err != nil || res.Hash != "h" || res.Width != 1200 {
		t.Fatal(res, err)
	}
	var apiErr *APIError
	if _, err := c.GetUsage(ctx); !errors.As(err, &apiErr) || apiErr.Code != "invalid_input" || apiErr.Status != 400 || apiErr.Detail[0].Field != "title" {
		t.Fatal(err)
	}
	var redir *RedirectError
	if _, err := c.ListTemplates(ctx); !errors.As(err, &redir) || redir.Status != 302 {
		t.Fatal(err)
	}
	if _, _, err := c.FreeRender(ctx, "x", "", ""); !errors.As(err, &apiErr) || apiErr.Code != "unknown_error" {
		t.Fatal(err)
	}
	if _, err := (&Client{BaseURL: srv.URL}).Render(ctx, RenderRequest{Template: "basic"}); !errors.As(err, &apiErr) || apiErr.Code != "unauthenticated" {
		t.Fatal(err)
	}
	if _, err := (&Client{APIKey: "k", BaseURL: "http://127.0.0.1:1"}).Render(ctx, RenderRequest{Template: "basic"}); err == nil {
		t.Fatal("want network error")
	} else if _, ok := err.(*NetworkError); !ok {
		t.Fatal(err)
	}
}

func TestRenderHTMLAndScreenshot(t *testing.T) {
	var lastPath, lastBody, lastAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		lastPath, lastBody, lastAuth = r.URL.Path, string(b), r.Header.Get("Authorization")
		w.WriteHeader(201)
		if r.URL.Path == "/v1/screenshot" {
			w.Write([]byte(`{"url":"https://ogmake.com/s/x.png","fullPage":true,"credits":3}`))
			return
		}
		w.Write([]byte(`{"url":"https://ogmake.com/s/y.png","credits":2}`))
	}))
	defer srv.Close()
	ctx := context.Background()
	c := &Client{APIKey: "og_live_x", BaseURL: srv.URL}
	h, err := c.RenderHTML(ctx, RenderHTMLRequest{HTML: "<h1>Hi</h1>", CSS: "h1{}", Width: 1200, Height: 630, Format: "webp"})
	if err != nil || h.Credits != 2 || lastPath != "/v1/images" || lastAuth != "Bearer og_live_x" {
		t.Fatal(h, err, lastPath)
	}
	wantJSON(t, lastBody, map[string]interface{}{"html": "<h1>Hi</h1>", "css": "h1{}", "width": 1200.0, "height": 630.0, "format": "webp"})
	if _, err := c.RenderHTML(ctx, RenderHTMLRequest{HTML: "<p>x</p>", Width: 100, Height: 100}); err != nil {
		t.Fatal(err)
	}
	wantJSON(t, lastBody, map[string]interface{}{"html": "<p>x</p>", "width": 100.0, "height": 100.0})
	s, err := c.Screenshot(ctx, ScreenshotRequest{URL: "https://example.com/", Width: 1280, Height: 800, FullPage: true, Format: "jpg"})
	if err != nil || s.Credits != 3 || !s.FullPage || lastPath != "/v1/screenshot" {
		t.Fatal(s, err, lastPath)
	}
	wantJSON(t, lastBody, map[string]interface{}{"url": "https://example.com/", "width": 1280.0, "height": 800.0, "fullPage": true, "format": "jpg"})
	if _, err := c.Screenshot(ctx, ScreenshotRequest{URL: "https://example.com/", Width: 1280}); err != nil {
		t.Fatal(err)
	}
	wantJSON(t, lastBody, map[string]interface{}{"url": "https://example.com/", "width": 1280.0})
	var apiErr *APIError
	if _, err := (&Client{BaseURL: srv.URL}).Screenshot(ctx, ScreenshotRequest{URL: "https://example.com/", Width: 1280}); !errors.As(err, &apiErr) || apiErr.Code != "unauthenticated" {
		t.Fatal(err)
	}
}

func wantJSON(t *testing.T, got string, want map[string]interface{}) {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(got), &m); err != nil || !reflect.DeepEqual(m, want) {
		t.Fatalf("body %s (err %v), want %v", got, err, want)
	}
}
