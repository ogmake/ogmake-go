// Package ogmake is a thin SDK for the ogmake OG image API (https://ogmake.com).
package ogmake

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	Version        = "0.1.0"
	DefaultBaseURL = "https://ogmake.com"
	maxFreeBytes   = 5 * 1024 * 1024
)

// ImageOptions are the optional control params shared by signed URLs and renders.
type ImageOptions struct {
	Format string // png | jpg | webp | pdf
	Preset string // og | x | square
	Scale  string // "1" | "2"
	V      string
	Bg     string
}

func (o ImageOptions) apply(dst map[string]string) {
	for k, v := range map[string]string{"format": o.Format, "preset": o.Preset, "scale": o.Scale, "v": o.V, "bg": o.Bg} {
		if v != "" {
			dst[k] = v
		}
	}
}

// SignedImageURL builds {base}/i/{keyID}/{sig}?{canonical}. signingSecret is the key's base64 secret.
// An empty baseURL means DefaultBaseURL.
func SignedImageURL(baseURL, keyID, signingSecret, template string, params map[string]string, opts ImageOptions) (string, error) {
	secret, err := DecodeSecret(signingSecret)
	if err != nil {
		return "", err
	}
	query := map[string]string{}
	for k, v := range params {
		query[k] = v
	}
	query["template"] = template
	opts.apply(query)
	canonical := CanonicalQuery(query)
	sig := SignQuery(secret, canonical)
	return trimBase(baseURL) + "/i/" + url.PathEscape(keyID) + "/" + sig + "?" + canonical, nil
}

// PublicImageURL builds the unsigned {base}/p/{keyID}/{templateID} URL.
func PublicImageURL(baseURL, keyID, templateID string) string {
	return trimBase(baseURL) + "/p/" + url.PathEscape(keyID) + "/" + url.PathEscape(templateID)
}

func trimBase(b string) string {
	if b == "" {
		b = DefaultBaseURL
	}
	return strings.TrimRight(b, "/")
}

// Client calls the ogmake API. Zero values are usable except APIKey for authenticated calls.
type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client // optional; redirects are never followed regardless
}

// RenderRequest is the POST /v1/images body.
type RenderRequest struct {
	Template string                 `json:"template"`
	Params   map[string]interface{} `json:"params,omitempty"`
	Format   string                 `json:"format,omitempty"`
	Preset   string                 `json:"preset,omitempty"`
	Scale    string                 `json:"scale,omitempty"`
	V        string                 `json:"v,omitempty"`
	Bg       string                 `json:"bg,omitempty"`
}

// RenderResult is the stored-render response.
type RenderResult struct {
	URL             string `json:"url"`
	Hash            string `json:"hash"`
	Cached          bool   `json:"cached"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	Format          string `json:"format"`
	Bytes           int    `json:"bytes"`
	TemplateVersion string `json:"templateVersion"`
}

// Render calls POST /v1/images (stored mode).
func (c *Client) Render(ctx context.Context, req RenderRequest) (*RenderResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	raw, _, err := c.do(ctx, http.MethodPost, "/v1/images", body, true)
	if err != nil {
		return nil, err
	}
	var out RenderResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RenderHTMLRequest is the raw-HTML POST /v1/images body (2 credits).
type RenderHTMLRequest struct {
	HTML   string `json:"html"`
	CSS    string `json:"css,omitempty"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format,omitempty"` // png | jpg | webp | pdf
}

// RenderHTMLResult is the raw-HTML render response.
type RenderHTMLResult struct {
	URL     string `json:"url"`
	Hash    string `json:"hash"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Format  string `json:"format"`
	Bytes   int    `json:"bytes"`
	Credits int    `json:"credits"`
}

// RenderHTML calls POST /v1/images with raw HTML.
func (c *Client) RenderHTML(ctx context.Context, req RenderHTMLRequest) (*RenderHTMLResult, error) {
	var out RenderHTMLResult
	if err := c.postJSON(ctx, "/v1/images", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ScreenshotRequest is the POST /v1/screenshot body (3 credits).
type ScreenshotRequest struct {
	URL      string `json:"url"`
	Width    int    `json:"width"`
	Height   int    `json:"height,omitempty"` // 0 = server default (800)
	FullPage bool   `json:"fullPage,omitempty"`
	Format   string `json:"format,omitempty"` // png | jpg | webp
}

// ScreenshotResult is the screenshot response.
type ScreenshotResult struct {
	URL      string `json:"url"`
	Hash     string `json:"hash"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	FullPage bool   `json:"fullPage"`
	Format   string `json:"format"`
	Bytes    int    `json:"bytes"`
	Credits  int    `json:"credits"`
}

// Screenshot calls POST /v1/screenshot for a public page.
func (c *Client) Screenshot(ctx context.Context, req ScreenshotRequest) (*ScreenshotResult, error) {
	var out ScreenshotResult
	if err := c.postJSON(ctx, "/v1/screenshot", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) postJSON(ctx context.Context, path string, in, out interface{}) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	raw, _, err := c.do(ctx, http.MethodPost, path, body, true)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// ListTemplates calls GET /v1/templates (no key needed); returns the raw JSON.
func (c *Client) ListTemplates(ctx context.Context) (json.RawMessage, error) {
	raw, _, err := c.do(ctx, http.MethodGet, "/v1/templates", nil, false)
	return raw, err
}

// GetUsage calls GET /v1/usage; returns the raw JSON.
func (c *Client) GetUsage(ctx context.Context) (json.RawMessage, error) {
	raw, _, err := c.do(ctx, http.MethodGet, "/v1/usage", nil, true)
	return raw, err
}

// FreeRender calls GET /v1/free/og and returns the image bytes and content type.
func (c *Client) FreeRender(ctx context.Context, title, description, site string) ([]byte, string, error) {
	q := url.Values{"title": {title}}
	if description != "" {
		q.Set("description", description)
	}
	if site != "" {
		q.Set("site", site)
	}
	raw, h, err := c.do(ctx, http.MethodGet, "/v1/free/og?"+q.Encode(), nil, false)
	if err != nil {
		return nil, "", err
	}
	ct := h.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(ct), "image/") {
		return nil, "", &APIError{Code: "unexpected_content_type", Status: 200}
	}
	return raw, ct, nil
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, auth bool) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, method, trimBase(c.BaseURL)+path, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "ogmake-go/"+Version)
	if auth {
		if c.APIKey == "" {
			return nil, nil, &APIError{Code: "unauthenticated", Status: 0}
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := http.Client{Timeout: 20 * time.Second}
	if c.HTTPClient != nil {
		hc = *c.HTTPClient
	}
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := hc.Do(req)
	if err != nil {
		return nil, nil, &NetworkError{Err: err}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxFreeBytes+1))
	if err != nil {
		return nil, nil, &NetworkError{Err: err}
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, nil, &RedirectError{Status: resp.StatusCode, Location: resp.Header.Get("Location")}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, toAPIError(raw, resp.StatusCode)
	}
	if len(raw) > maxFreeBytes {
		return nil, nil, &APIError{Code: "response_too_large", Status: resp.StatusCode}
	}
	return raw, resp.Header, nil
}
