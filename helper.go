package apirouter

import (
	"context"
	"io"
	"io/fs"
	"net/http"
)

// GetRequestBody returns the current request's body if any, or an error
func GetRequestBody(ctx context.Context) (io.ReadCloser, error) {
	req, ok := ctx.Value("http_request").(*http.Request)
	if !ok || req == nil {
		return nil, fs.ErrNotExist
	}
	if req.GetBody == nil {
		return nil, fs.ErrNotExist
	}
	return req.GetBody()
}

// GetHeader returns the requested header or an empty string if not found
func GetHeader(ctx context.Context, hdr string) string {
	req, ok := ctx.Value("http_request").(*http.Request)
	if !ok || req == nil {
		return ""
	}
	return req.Header.Get(hdr)
}

// SecurePost ensures request was a POST request and has the required headers
func SecurePost(ctx context.Context) error {
	c := &Context{}
	ctx.Value(&c)

	if c == nil {
		return ErrInsecureRequest
	}
	if c.verb != "POST" {
		return ErrInsecureRequest
	}
	// check if tokenOk is set
	if !c.csrfOk {
		return ErrInsecureRequest
	}
	return nil
}
