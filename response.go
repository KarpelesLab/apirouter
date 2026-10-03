package apirouter

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/KarpelesLab/webutil"
	"github.com/fxamacker/cbor/v2"
)

// ResponseSink is an interface for sending API responses to different backends.
// Implementations can send responses to HTTP response writers, WebSocket connections,
// encoders, or any other output destination.
type ResponseSink interface {
	// SendResponse sends the response to the underlying backend.
	SendResponse(*Response) error
}

// Response represents an API response with metadata and payload.
// It can represent success, error, redirect, or progress responses.
type Response struct {
	Result       string  `json:"result"` // error|success|redirect
	Error        string  `json:"error,omitempty"`
	Token        string  `json:"token,omitempty"`
	ErrorInfo    any     `json:"error_info,omitempty"`
	Code         int     `json:"code,omitempty"`
	Debug        string  `json:"debug,omitempty"`
	RequestId    string  `json:"request_id,omitempty"`
	Time         float64 `json:"time"`
	Data         any     `json:"data"`
	RedirectURL  string  `json:"redirect_url,omitempty"`
	RedirectCode int     `json:"redirect_code,omitempty"`
	QueryId      any     `json:"query_id,omitempty"`
	err          error
	ctx          *Context
	subhandler   http.HandlerFunc
}

func (c *Context) errorResponse(err error) *Response {
	code := webutil.HTTPStatus(err)
	if code == 0 {
		code = http.StatusInternalServerError
	}
	if e, ok := err.(*webutil.Redirect); ok {
		res := &Response{
			Result:       "redirect",
			RedirectURL:  e.URL.String(),
			RedirectCode: e.Code,
			Time:         float64(time.Since(c.start)) / float64(time.Second),
			RequestId:    c.reqid,
			QueryId:      c.qid,
			err:          e,
			ctx:          c,
		}
		return res
	}

	res := &Response{
		Result:    "error",
		Error:     err.Error(),
		Code:      code,
		Time:      float64(time.Since(c.start)) / float64(time.Second),
		RequestId: c.reqid,
		QueryId:   c.qid,
		err:       err,
		ctx:       c,
	}
	if obj, ok := err.(*Error); ok {
		res.Token = obj.Token
		res.ErrorInfo = obj.Info
	}
	return res
}

func (c *Context) progressResponse(data any) *Response {
	res := &Response{
		Result:    "progress",
		Time:      float64(time.Since(c.start)) / float64(time.Second),
		RequestId: c.reqid,
		QueryId:   c.qid,
		Data:      data,
		ctx:       c,
	}
	for _, h := range c.responseHooks() {
		h(res)
	}

	return res
}

// Response executes the request and generates a response object
func (c *Context) Response() (res *Response, err error) {

	defer func() {
		if e := recover(); e != nil {
			stack := debug.Stack()
			slog.ErrorContext(c, fmt.Sprintf("[api] panic in %s: %s\nStack\n%s", c.path, e, stack), "event", "apirouter:response:panic", "category", "go.panic")
			err = fmt.Errorf("panic: %s", e)
			res = &Response{
				Result:    "error",
				Error:     fmt.Sprintf("panic: %s", e),
				Code:      http.StatusInternalServerError,
				Debug:     string(stack),
				Time:      float64(time.Since(c.start)) / float64(time.Second),
				RequestId: c.reqid,
				QueryId:   c.qid,
				err:       err,
				ctx:       c,
			}
		}
	}()

	for _, h := range c.requestHooks() {
		if err = h(c); err != nil {
			res = c.errorResponse(err)
			return
		}
	}

	code := http.StatusOK
	var val any
	val, err = c.Call() // perform the actual call

	if err != nil {
		res = c.errorResponse(err)
		for _, h := range c.responseHooks() {
			if err := h(res); err != nil {
				return c.errorResponse(err), err
			}
		}
		return
	}

	if obj, ok := val.(*Response); ok {
		// already a response object
		res = obj
		if res.ctx == nil {
			res.ctx = c
		}
		res.Time = float64(time.Since(c.start)) / float64(time.Second)
		for _, h := range c.responseHooks() {
			h(res)
		}
		return
	}

	res = &Response{
		Result:    "success",
		Code:      code,
		Time:      float64(time.Since(c.start)) / float64(time.Second),
		RequestId: c.reqid,
		QueryId:   c.qid,
		Data:      val,
		ctx:       c,
	}
	for _, h := range c.responseHooks() {
		h(res)
	}
	return
}

// requestHooks returns the global request hooks followed by the ones of the
// router serving the request.
func (c *Context) requestHooks() []RequestHook {
	if c.router == nil || len(c.router.RequestHooks) == 0 {
		return RequestHooks
	}
	return append(append([]RequestHook{}, RequestHooks...), c.router.RequestHooks...)
}

// responseHooks returns the global response hooks followed by the ones of
// the router serving the request.
func (c *Context) responseHooks() []ResponseHook {
	if c.router == nil || len(c.router.ResponseHooks) == 0 {
		return ResponseHooks
	}
	return append(append([]ResponseHook{}, ResponseHooks...), c.router.ResponseHooks...)
}

// Err returns the error this response was generated from, if any.
func (r *Response) Err() error {
	return r.err
}

func (r *Response) getResponseData() any {
	res := make(map[string]any)
	if r.ctx.extra != nil {
		for k, v := range r.ctx.extra {
			res[k] = v
		}
	}
	res["result"] = r.Result
	if r.Error != "" {
		res["error"] = r.Error
		res["code"] = r.Code
	}
	res["time"] = r.Time
	res["data"] = r.Data
	res["request_id"] = r.RequestId
	if r.RedirectURL != "" {
		res["redirect_url"] = r.RedirectURL
		if r.RedirectCode != 0 {
			res["redirect_code"] = r.RedirectCode
		}
	}
	if r.Token != "" {
		res["token"] = r.Token
	}
	if r.ErrorInfo != nil {
		res["error_info"] = r.ErrorInfo
	}
	if r.QueryId != nil {
		res["query_id"] = r.QueryId
	}
	if rt := r.ctx.router; rt != nil {
		for _, h := range rt.EnvelopeHooks {
			h(r, res)
		}
	}

	return res
}

// MarshalJSON implements json.Marshaler for Response.
// It marshals the response data including any extra context data.
func (r *Response) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.getResponseData())
}

// GetContext returns the Context associated with this response.
func (r *Response) GetContext() *Context {
	return r.ctx
}

// jsonOpts returns the json options to use when marshaling this response, which
// hide protected fields unless the context allows showing them
func (r *Response) jsonOpts() json.Options {
	if r.ctx.showProt {
		return json.JoinOptions()
	}
	return publicJsonOpts
}

// ServeHTTP implements http.Handler for Response, allowing it to be used directly
// as an HTTP handler. It writes the response with appropriate headers for CORS,
// caching, and content negotiation (JSON or CBOR based on Accept header).
func (r *Response) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if h := r.subhandler; h != nil {
		h(rw, req)
		return
	}

	// check req for HTTP Query flags: raw
	_, raw := r.ctx.flags["raw"]
	hdr := rw.Header()

	// add standard headers for API responses (no cache, cors)
	if c, ok := r.ctx.extra["cache"].(time.Duration); ok && c > 0 {
		secs := int64(c / time.Second)
		hdr.Set("Cache-Control", fmt.Sprintf("public,max-age=%d", secs)) // ,immutable
		hdr.Set("Expires", time.Now().Add(c).Format(time.RFC1123))
		hdr.Set("X-Accel-Expires", strconv.FormatInt(secs, 10))
	} else {
		hdr.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		hdr.Set("Expires", time.Now().Add(-365*86400*time.Second).Format(time.RFC1123))
	}
	if rt := r.ctx.router; rt == nil || !rt.DisableCORS {
		// access-control-allow-credentials: true
		// access-control-allow-origin: *
		hdr.Set("Access-Control-Allow-Credentials", "true")
		if origin := req.Header.Get("Origin"); origin != "" {
			hdr.Set("Vary", "Accept-Encoding,Origin")
			hdr.Set("Access-Control-Allow-Origin", origin)
		} else {
			hdr.Set("Access-Control-Allow-Origin", "*")
		}
	}
	// For OPTIONS we also add (at a higher level):
	// Access-Control-Allow-Headers: Authorization, Content-Type
	// Access-Control-Max-Age: 86400
	// Access-Control-Allow-Methods: POST, GET, OPTIONS, PUT, DELETE, PATCH
	// Allow: POST, GET, OPTIONS

	// headers set by the endpoint (Context.Header)
	for k, v := range r.ctx.header {
		hdr[k] = v
	}

	if raw {
		if r.err != nil {
			var h http.Handler
			if errors.As(r.err, &h) {
				// errors that know how to answer (redirects, OPTIONS...)
				h.ServeHTTP(rw, req)
				return
			}
			// other errors are sent as a normal error response
			if err := r.writeObject(rw, r.getResponseData()); err != nil {
				webutil.ErrorToHttpHandler(err).ServeHTTP(rw, req)
			}
			return
		}
		if mime, ok := r.ctx.extra["mime"].(string); ok {
			hdr.Set("Content-Type", mime)
		}

		switch v := r.Data.(type) {
		case string:
			if v == "" {
				rw.WriteHeader(http.StatusNoContent)
				return
			}
			if hdr.Get("Content-Type") == "" {
				hdr.Set("Content-Type", "text/plain; charset=utf-8")
			}
			rw.Write([]byte(v))
			return
		case []byte:
			rw.Write(v)
			return
		case io.Reader:
			_, err := io.Copy(rw, v)
			if fc, ok := v.(io.Closer); ok {
				fc.Close()
			}
			if err != nil {
				webutil.ErrorToHttpHandler(err).ServeHTTP(rw, req)
			}
			return
		default:
			// encode to json
			err := r.writeObject(rw, v)
			if err != nil {
				webutil.ErrorToHttpHandler(err).ServeHTTP(rw, req)
			}
			return
		}
	}

	// send response normally
	err := r.writeObject(rw, r.getResponseData())
	if err != nil {
		webutil.ErrorToHttpHandler(err).ServeHTTP(rw, req)
	}
}

func (r *Response) writeObject(rw http.ResponseWriter, obj any) error {
	typ := r.ctx.selectAcceptedType("application/json", "application/cbor")

	switch typ {
	case "application/json":
		_, pretty := r.ctx.flags["pretty"]
		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Code != 0 {
			rw.WriteHeader(r.Code)
		}
		opts := []json.Options{r.jsonOpts()}
		if pretty {
			opts = append(opts, jsontext.WithIndent("    "))
		}
		return json.MarshalEncode(jsontext.NewEncoder(rw, opts...), obj)
	case "application/cbor":
		rw.Header().Set("Content-Type", "application/cbor")
		if r.Code != 0 {
			rw.WriteHeader(r.Code)
		}
		enc := cbor.NewEncoder(rw)
		return enc.Encode(obj)
	default:
		return errors.New("could not encode object (should never happen)")
	}
}
