# ogmake-go

Thin Go SDK for the [ogmake](https://ogmake.com) OG image API. Standard library only.

## Install

```sh
go get github.com/IgorBaranov/ogmake-go   # not yet published
```

## Examples

Signed URL (computed locally; the signing secret never leaves your process):

```go
u, err := ogmake.SignedImageURL("", "KEY_ID", "BASE64_SIGNING_SECRET", "basic",
	map[string]string{"title": "Hello World"}, ogmake.ImageOptions{Format: "png"})
```

Public (unsigned) URL for a key marked public with the template allow-listed:

```go
u := ogmake.PublicImageURL("", "KEY_ID", "basic") + "?title=Hello"
```

Stored render with typed errors:

```go
c := &ogmake.Client{APIKey: "og_live_..."}
res, err := c.Render(ctx, ogmake.RenderRequest{Template: "basic", Params: map[string]interface{}{"title": "Hello"}})
var apiErr *ogmake.APIError
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.Code, apiErr.Status, apiErr.Detail)
}
```

Errors: `*APIError` (`Code`, `Status`, `Detail`), `*NetworkError`, `*RedirectError` (redirects are never followed). Raw HTML render (2 credits): `c.RenderHTML(ctx, ogmake.RenderHTMLRequest{HTML: html, Width: 1200, Height: 630})`. Page screenshot (3 credits): `c.Screenshot(ctx, ogmake.ScreenshotRequest{URL: "https://example.com/", Width: 1280})`. Also `ListTemplates`, `GetUsage`, `FreeRender`.

Tests: `go test ./...`. MIT licensed. Support: support@ogmake.com
