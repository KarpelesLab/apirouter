package apirouter

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// HTTP is the main HTTP handler for the API router.
// It can be used directly as an http.Handler or http.HandlerFunc.
// The handler parses incoming requests, routes them to the appropriate
// API endpoint via the pobj framework, and returns JSON or CBOR responses.
//
// Example usage:
//
//	http.Handle("/_api/", http.StripPrefix("/_api", apirouter.HTTP))
var HTTP = http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
	ctx, err := NewHttp(rw, req)
	if err != nil {
		res := ctx.errorResponse(err)
		res.ServeHTTP(rw, req)
		return
	}
	res, _ := ctx.Response()
	res.ServeHTTP(rw, req)
})

type optionsResponder struct {
	allowedMethods []string
}

func (o *optionsResponder) Error() string {
	return "Options responder"
}

func (o *optionsResponder) getAllowedMethods() string {
	if o.allowedMethods == nil {
		return "POST, GET, OPTIONS, PUT, DELETE, PATCH"
	}
	return strings.Join(o.allowedMethods, ", ")
}

func (o *optionsResponder) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// set headers, return no body
	rw.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	rw.Header().Set("Access-Control-Max-Age", "86400")
	rw.Header().Set("Access-Control-Allow-Methods", o.getAllowedMethods())
	rw.WriteHeader(http.StatusNoContent)
}

// GetDomainForRequest returns the domain name for an HTTP request.
// It checks the Sec-Original-Host header first (for proxy scenarios),
// then falls back to the Host header, stripping any port number.
// Returns "_default" if no domain can be determined.
func GetDomainForRequest(req *http.Request) string {
	if originalHost := req.Header.Get("Sec-Original-Host"); originalHost != "" {
		if host, _, _ := net.SplitHostPort(originalHost); host != "" {
			return host
		}
		return originalHost
	}
	if req.Host != "" {
		host, _, _ := net.SplitHostPort(req.Host)
		if host != "" {
			return host
		}
		return req.Host
	}

	// fallback
	return "_default"
}

// GetPrefixForRequest returns a URL that can be used to address the server directly.
// It constructs the URL from the request's scheme, domain, and any path prefix.
// It handles Sec-Original-Host and Sec-Access-Prefix headers for proxy scenarios.
func GetPrefixForRequest(req *http.Request) *url.URL {
	u := &url.URL{Host: GetDomainForRequest(req)}
	// determine if we got https
	if req.TLS == nil {
		u.Scheme = "http"
	} else {
		u.Scheme = "https"
	}
	// check if we have a prefix
	if pfx := req.Header.Get("Sec-Access-Prefix"); pfx != "" {
		if !strings.HasPrefix(pfx, "/") {
			pfx = "/" + pfx
		}
		u.Path = pfx
	} else {
		u.Path = "/"
	}

	return u
}
