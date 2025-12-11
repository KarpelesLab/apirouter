package apirouter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/KarpelesLab/pjson"
	"github.com/KarpelesLab/webutil"
	"github.com/fxamacker/cbor/v2"
)

type ResponseSink interface {
	SendResponse(*Response) error
}

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
	for _, h := range ResponseHooks {
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

	for _, h := range RequestHooks {
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
		for _, h := range ResponseHooks {
			if err := h(res); err != nil {
				return c.errorResponse(err), err
			}
		}
		return
	}

	if obj, ok := val.(*Response); ok {
		// already a response object
		res = obj
		res.Time = float64(time.Since(c.start)) / float64(time.Second)
		for _, h := range ResponseHooks {
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
	for _, h := range ResponseHooks {
		h(res)
	}
	return
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

	return res
}

func (r *Response) MarshalJSON() ([]byte, error) {
	return pjson.Marshal(r.getResponseData())
}

func (r *Response) MarshalContextJSON(ctx context.Context) ([]byte, error) {
	return pjson.MarshalContext(ctx, r.getResponseData())
}

func (r *Response) GetContext() *Context {
	return r.ctx
}

// getJsonCtx returns a context to pass to MarshalContext that may hide some values
func (r *Response) getJsonCtx() context.Context {
	if r.ctx.showProt {
		return r.ctx
	}
	return pjson.ContextPublic(r.ctx)
}

func (r *Response) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if h := r.subhandler; h != nil {
		h(rw, req)
		return
	}

	// check req for HTTP Query flags: raw
	_, raw := r.ctx.flags["raw"]

	// add standard headers for API responses (no cache, cors)
	if c, ok := r.ctx.extra["cache"].(time.Duration); ok && c > 0 {
		secs := int64(c / time.Second)
		rw.Header().Set("Cache-Control", fmt.Sprintf("public,max-age=%d", secs)) // ,immutable
		rw.Header().Set("Expires", time.Now().Add(c).Format(time.RFC1123))
		rw.Header().Set("X-Accel-Expires", strconv.FormatInt(secs, 10))
	} else {
		rw.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		rw.Header().Set("Expires", time.Now().Add(-365*86400*time.Second).Format(time.RFC1123))
	}
	// access-control-allow-credentials: true
	// access-control-allow-origin: *
	rw.Header().Set("Access-Control-Allow-Credentials", "true")
	if origin := req.Header.Get("Origin"); origin != "" {
		rw.Header().Set("Vary", "Accept-Encoding,Origin")
		rw.Header().Set("Access-Control-Allow-Origin", origin)
	} else {
		rw.Header().Set("Access-Control-Allow-Origin", "*")
	}
	// For OPTIONS we also add (at a higher level):
	// Access-Control-Allow-Headers: Authorization, Content-Type
	// Access-Control-Max-Age: 86400
	// Access-Control-Allow-Methods: POST, GET, OPTIONS, PUT, DELETE, PATCH
	// Allow: POST, GET, OPTIONS

	if raw {
		if r.err != nil {
			webutil.ErrorToHttpHandler(r.err).ServeHTTP(rw, req)
			return
		}
		if mime, ok := r.ctx.extra["mime"].(string); ok {
			rw.Header().Set("Content-Type", mime)
		}

		switch v := r.Data.(type) {
		case string:
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
		enc := pjson.NewEncoderContext(r.getJsonCtx(), rw)
		if pretty {
			enc.SetIndent("", "    ")
		}
		return enc.Encode(obj)
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
