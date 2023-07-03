package apirouter

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/KarpelesLab/pjson"
	"github.com/KarpelesLab/webutil"
)

type Response struct {
	Result       string   `json:"result"` // error|success|redirect
	Error        string   `json:"error,omitempty"`
	Token        string   `json:"token,omitempty"`
	Code         int      `json:"code,omitempty"`
	Debug        string   `json:"debug,omitempty"`
	RequestId    string   `json:"request_id,omitempty"`
	Time         float64  `json:"time"`
	Data         any      `json:"data"`
	RedirectURL  *url.URL `json:"redirect_url,omitempty"`
	RedirectCode int      `json:"redirect_code,omitempty"`
	err          error
	ctx          *Context
}

func (c *Context) errorResponse(start time.Time, err error) *Response {
	code := webutil.HTTPStatus(err)
	if code == 0 {
		code = http.StatusInternalServerError
	}
	if e, ok := err.(*webutil.Redirect); ok {
		res := &Response{
			Result:       "redirect",
			RedirectURL:  e.URL,
			RedirectCode: e.Code,
			Time:         float64(time.Since(start)) / float64(time.Second),
			RequestId:    c.reqid,
			err:          e,
			ctx:          c,
		}
		return res
	}

	res := &Response{
		Result:    "error",
		Error:     err.Error(),
		Code:      code,
		Time:      float64(time.Since(start)) / float64(time.Second),
		RequestId: c.reqid,
		err:       err,
		ctx:       c,
	}
	if obj, ok := err.(*Error); ok {
		res.Token = obj.Token
	}
	return res
}

func (c *Context) Response() (res *Response, err error) {
	start := time.Now()

	defer func() {
		if e := recover(); e != nil {
			stack := debug.Stack()
			log.Printf("[api] panic in %s: %s\nStack\n%s", c.path, e, stack)
			res = &Response{
				Result:    "error",
				Error:     fmt.Sprintf("panic: %s", e),
				Code:      http.StatusInternalServerError,
				Debug:     string(stack),
				Time:      float64(time.Since(start)) / float64(time.Second),
				RequestId: c.reqid,
				err:       fmt.Errorf("panic: %s", err),
				ctx:       c,
			}
			err = fmt.Errorf("panic: %s", err)
		}
	}()

	for _, h := range RequestHooks {
		if err = h(c); err != nil {
			res = c.errorResponse(start, err)
			return
		}
	}

	code := http.StatusOK
	var val any
	val, err = c.Call() // perform the actual call

	if err != nil {
		res = c.errorResponse(start, err)
		for _, h := range ResponseHooks {
			if err := h(res); err != nil {
				return nil, err
			}
		}
		return
	}

	if obj, ok := val.(*Response); ok {
		// already a response object
		res.Time = float64(time.Since(start)) / float64(time.Second)
		res = obj
		for _, h := range ResponseHooks {
			h(res)
		}
		return
	}

	res = &Response{
		Result:    "success",
		Code:      code,
		Time:      float64(time.Since(start)) / float64(time.Second),
		RequestId: c.reqid,
		Data:      val,
		ctx:       c,
	}
	for _, h := range ResponseHooks {
		h(res)
	}
	return
}

func (r *Response) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	r.serveWithContext(r.ctx, rw, req)
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
	if r.RedirectURL != nil {
		res["redirect_url"] = r.RedirectURL
		if r.RedirectCode != 0 {
			res["redirect_code"] = r.RedirectCode
		}
	}
	if r.Token != "" {
		res["token"] = r.Token
	}

	return res
}

func (r *Response) GetContext() *Context {
	return r.ctx
}

//go:noinline
func (r *Response) serveWithContext(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
	// check req for HTTP Query flags: raw & pretty
	_, raw := r.ctx.flags["raw"]
	_, pretty := r.ctx.flags["pretty"]

	// add standard headers for API respsones (no cache, cors)
	rw.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	rw.Header().Set("Expires", time.Now().Add(-365*86400*time.Second).Format(time.RFC1123))
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
			rw.Header().Set("Content-Type", "application/json; charset=utf-8")
			enc := pjson.NewEncoderContext(r.ctx, rw)
			if pretty {
				enc.SetIndent("", "    ")
			}
			err := enc.Encode(v)
			if err != nil {
				webutil.ErrorToHttpHandler(err).ServeHTTP(rw, req)
			}
			return
		}
	}

	// send response normally
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Code != 0 {
		rw.WriteHeader(r.Code)
	}
	enc := pjson.NewEncoderContext(r.ctx, rw)
	if pretty {
		enc.SetIndent("", "    ")
	}

	err := enc.Encode(r.getResponseData())
	if err != nil {
		webutil.ErrorToHttpHandler(err).ServeHTTP(rw, req)
	}
	runtime.KeepAlive(ctx)
}
