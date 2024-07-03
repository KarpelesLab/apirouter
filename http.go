package apirouter

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// apirouter.HTTP can be used as a handler function, or as a handler
// via http.HandlerFunc(apirouter.HTTP)
func HTTP(rw http.ResponseWriter, req *http.Request) {
	ctx, err := NewHttp(rw, req)
	if err != nil {
		res := ctx.errorResponse(err)
		res.ServeHTTP(rw, req)
		return
	}
	res, _ := ctx.Response()
	res.ServeHTTP(rw, req)
}

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

// GetDomainForRequest will return the domain for a given http.Request, handling cases with redirects
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

// GetPrefixForRequest can be used to obtain the prefix for a given request and will
// be able to address the local server directly. It'll handle Sec-Original-Host and
// Sec-Access-Prefix headers
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
