package apirouter

import (
	"context"
	"net/http"

	"github.com/KarpelesLab/pobj"
)

// Router is an http.Handler serving an API like HTTP does, with its own
// object tree and hooks. Several Routers can be used in the same program
// (for example one per API server, or one per test), each routing requests
// into its own tree created with pobj.NewRoot.
//
// Example:
//
//	root := pobj.NewRoot()
//	root.RegisterMethod("User:login", login).SetVerbs("POST")
//	rt := &apirouter.Router{Root: root}
//	http.Handle("/_rest/", http.StripPrefix("/_rest", rt))
type Router struct {
	// Root is the object tree requests are routed into. When nil, the
	// global tree (pobj.Root()) is used.
	Root *pobj.Object

	// RequestHooks run after the global RequestHooks, for requests served
	// by this router only.
	RequestHooks []RequestHook

	// ResponseHooks run after the global ResponseHooks, for requests
	// served by this router only.
	ResponseHooks []ResponseHook

	// EnvelopeHooks can modify the response envelope (the object encoded
	// as JSON or CBOR, with keys such as "result", "data" or "error")
	// right before it is sent, to follow a given API convention.
	EnvelopeHooks []EnvelopeHook

	// DisableCORS disables the Access-Control-* response headers, for APIs
	// that must not be readable from other origins (for example when they
	// are authenticated by cookies).
	DisableCORS bool
}

// EnvelopeHook is a function type that can modify the response envelope
// env of r (add, change or delete keys) before it is encoded. It is called
// for normal responses, and for error responses in raw mode.
type EnvelopeHook func(r *Response, env map[string]any)

type ctxRouterKey struct{}

// ServeHTTP implements http.Handler.
func (rt *Router) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	req = req.WithContext(context.WithValue(req.Context(), ctxRouterKey{}, rt))
	HTTP.ServeHTTP(rw, req)
}

// getRouter returns the Router serving ctx, or nil.
func getRouter(ctx context.Context) *Router {
	if ctx == nil {
		return nil
	}
	rt, _ := ctx.Value(ctxRouterKey{}).(*Router)
	return rt
}

// root returns the object tree the context routes into.
func (c *Context) root() *pobj.Object {
	if c.router != nil && c.router.Root != nil {
		return c.router.Root
	}
	return pobj.Root()
}

// GetRouter returns the Router serving this request, or nil when the
// request is served by HTTP or another transport without a Router.
func (c *Context) GetRouter() *Router {
	return c.router
}
