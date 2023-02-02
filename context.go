package apirouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/KarpelesLab/webutil"
)

type Context struct {
	context.Context

	path string // eg. "A/b:c"
	verb string // "GET", etc

	req    *http.Request       // can be nil
	rw     http.ResponseWriter // can be nil
	params map[string]any      // parameters passed from POST?
	get    map[string]any      // GET parameters (used for _ctx, etc)
	flags  map[string]bool     // flags, such as "raw" or "pretty"
	extra  map[string]any      // extra values in response

	objects map[string]any
}

func New(ctx context.Context, path, verb string) *Context {
	if ctx == nil {
		ctx = context.Background()
	}

	res := &Context{
		Context: ctx,
		path:    strings.TrimLeft(path, "/"),
		verb:    verb,
		objects: make(map[string]any),
		flags:   make(map[string]bool),
	}

	return res
}

func NewHttp(rw http.ResponseWriter, req *http.Request) (*Context, error) {
	res := &Context{
		Context: req.Context(),
		path:    strings.TrimLeft(req.URL.Path, "/"),
		verb:    req.Method,
		objects: make(map[string]any),
		flags:   make(map[string]bool),
	}

	err := res.SetHttp(rw, req)
	return res, err
}

func (c *Context) Value(v any) any {
	switch k := v.(type) {
	case **Context:
		*k = c
		return c
	default:
		return c.Context.Value(v)
	}
}

func (c *Context) SetParams(v map[string]any) {
	c.params = v
}

func (c *Context) SetParam(name string, v any) {
	if c.params == nil {
		c.params = make(map[string]any)
	}
	c.params[name] = v
}

func (c *Context) GetParams() any {
	return c.params
}

func (c *Context) GetParam(v string) any {
	if v == "" {
		return c.params
	}
	s := strings.Split(v, ".")
	var res any
	res = c.params

	for _, k := range s {
		// TODO detect if k is an int?
		if sub, ok := res.(map[string]any); ok {
			if res, ok = sub[k]; ok {
				continue
			} else {
				return nil
			}
		} else {
			return nil
		}
	}
	return res
}

func (c *Context) GetQuery(v string) any {
	return c.get[v]
}

func (c *Context) GetParamTo(v string, obj any) bool {
	// TODO
	return false
}

func (c *Context) SetPath(p string) {
	c.path = p
}

func (c *Context) GetPath() string {
	return c.path
}

func (c *Context) SetExtraResponse(k string, v any) {
	c.extra[k] = v
}

func (c *Context) GetExtraResponse(k string) any {
	return c.extra[k]
}

func (c *Context) GetObject(typ string) any {
	return c.objects[typ]
}

func (c *Context) SetHttp(rw http.ResponseWriter, req *http.Request) error {
	c.req = req
	c.rw = rw
	c.verb = req.Method
	c.get = webutil.ParsePhpQuery(req.URL.RawQuery)

	if _, raw := c.get["raw"]; raw {
		c.flags["raw"] = true
	}
	if _, pretty := c.get["pretty"]; pretty {
		c.flags["pretty"] = true
	}

	// try to parse params
	if c.params != nil {
		return nil
	}

	switch c.req.Method {
	case "POST", "PATCH", "PUT":
		ct, params, err := mime.ParseMediaType(c.req.Header.Get("Content-Type"))
		if err != nil {
			return err
		}

		body := c.req.Body
		if c.req.GetBody != nil {
			body, err = c.req.GetBody()
			if err != nil {
				return err
			}
		}
		reader := io.LimitReader(body, int64(10<<20)+1) // 10MB

		switch ct {
		case "application/json":
			// parse json
			dec := json.NewDecoder(reader)
			dec.UseNumber()
			return dec.Decode(&c.params)
		case "application/x-www-form-urlencoded":
			// parse url encoded
			b, e := io.ReadAll(reader)
			if e != nil {
				return e
			}
			p := webutil.ParsePhpQuery(string(b))
			if v, ok := p["_"]; ok {
				// _ contains json data, and overwrites any other parameter
				if v, ok := v.(string); ok {
					return json.Unmarshal([]byte(v), &c.params)
				}
			}
			c.params = p
			return nil
		case "multipart/form-data":
			// params should contain boundary
			boundary, ok := params["boundary"]
			if !ok {
				return http.ErrMissingBoundary
			}
			r := multipart.NewReader(reader, boundary)

			p := make(map[string]any)

			for {
				part, err := r.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				name := part.FormName()
				if name == "" {
					// ignore?
					continue
				}

				filename := part.FileName()

				b := &bytes.Buffer{}
				_, err = io.Copy(b, part)
				if err != nil {
					return err
				}

				if filename == "" {
					// normal value
					p[name] = b.String()
					continue
				}

				p[name] = map[string]any{"filename": filename, "data": b.Bytes()}
			}
			if v, ok := p["_"]; ok {
				// _ contains json data, and overwrites any other parameter
				if v, ok := v.(string); ok {
					return json.Unmarshal([]byte(v), &c.params)
				}
			}
			c.params = p
			return nil
		default:
			// unsupported body
			return nil
		}
	}

	// use GET
	if v, ok := c.get["_"]; ok {
		// _ contains json data, and overwrites any other parameter
		if v, ok := v.(string); ok {
			return json.Unmarshal([]byte(v), &c.params)
		}
	}
	return nil
}

// NewRequest returns a http request for this context (for example for forwarding, etc)
func (c *Context) NewRequest(target string) (*http.Request, error) {
	// target is for example http://localhost/_rest/, so it becomes http://localhost/_rest/A/B:c
	if target == "" {
		if c.req == nil {
			return nil, errors.New("missing target")
		}
		target = (&url.URL{Scheme: c.req.URL.Scheme, Host: c.req.URL.Host}).String()
	}
	target = path.Join(target, c.path)
	var targetQuery []string
	var body []byte
	headers := make(http.Header)

	if c.params != nil {
		js, err := json.Marshal(c.params)
		if err != nil {
			return nil, err
		}
		switch c.verb {
		case "POST", "PATCH", "PUT":
			// pass c.params in body
			body = js
			headers.Add("Content-Type", "application/json; charset=utf-8")
		default:
			// pass c.params in GET (targetQuery)
			targetQuery = append(targetQuery, "_="+url.QueryEscape(string(js)))
		}
	}

	var bodyR io.Reader
	if body != nil {
		bodyR = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(c, c.verb, target, bodyR)
	if err != nil {
		return nil, err
	}

	// tweak req
	if c.req != nil {
		// copy values from original request
		for k, v := range c.req.Header {
			req.Header[k] = v
		}
	}
	for k, v := range headers {
		req.Header[k] = v
	}

	return req, nil
}

func (c *Context) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	res, _ := c.Response()
	res.ServeHTTP(rw, req)
}
