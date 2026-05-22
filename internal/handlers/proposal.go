package handlers

import (
	_ "embed"

	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

//go:embed proposal.html
var proposalHTML []byte

// ProjectProposal serves the public sales proposal page used to pitch this
// platform to prospective customers (e.g. Bangla B2BB2C). The HTML is
// fully self-contained — CSS, fonts, and screenshot mockups are inlined —
// so it can be opened or shared without depending on any other asset.
//
// Public route, no auth.
func (a *App) ProjectProposal(r *fastglue.Request) error {
	r.RequestCtx.Response.Header.Set("Content-Type", "text/html; charset=utf-8")
	r.RequestCtx.Response.Header.Set("Cache-Control", "public, max-age=300")
	r.RequestCtx.SetStatusCode(fasthttp.StatusOK)
	_, _ = r.RequestCtx.Write(proposalHTML)
	return nil
}
