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
