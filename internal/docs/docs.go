// Package docs serves the OpenAPI spec and a Swagger UI page at /docs.
//
// The spec lives in openapi.yaml next to this file; the HTML is a tiny
// shim around the swagger-ui-dist CDN bundle. Both are embedded into the
// binary so the docs ship with the app and require no extra files at
// runtime.
package docs

import (
	_ "embed"

	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

//go:embed openapi.yaml
var openAPISpec []byte

//go:embed index.html
var indexHTML []byte

//go:embed websocket.html
var webSocketHTML []byte

// IndexHandler serves the Swagger UI HTML at /docs.
func IndexHandler(r *fastglue.Request) error {
	r.RequestCtx.SetContentType("text/html; charset=utf-8")
	r.RequestCtx.SetStatusCode(fasthttp.StatusOK)
	r.RequestCtx.SetBody(indexHTML)
	return nil
}

// SpecHandler serves the raw OpenAPI YAML at /docs/openapi.yaml.
// Served as text/yaml so curl-ing it prints to the terminal cleanly.
func SpecHandler(r *fastglue.Request) error {
	r.RequestCtx.SetContentType("application/yaml; charset=utf-8")
	r.RequestCtx.SetStatusCode(fasthttp.StatusOK)
	r.RequestCtx.SetBody(openAPISpec)
	return nil
}

// WebSocketHandler serves the WebSocket protocol reference at /docs/websocket.
// OpenAPI doesn't model WebSockets, so this is a hand-written HTML appendix.
func WebSocketHandler(r *fastglue.Request) error {
	r.RequestCtx.SetContentType("text/html; charset=utf-8")
	r.RequestCtx.SetStatusCode(fasthttp.StatusOK)
	r.RequestCtx.SetBody(webSocketHTML)
	return nil
}
