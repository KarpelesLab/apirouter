package apirouter_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/KarpelesLab/apirouter"
	"github.com/KarpelesLab/pobj"
	"github.com/KarpelesLab/webutil"
)

type thing struct {
	ID string `json:"id"`
}

// ApiHandle answers every verb on "Thing/<id>".
func (t *thing) ApiHandle(ctx *apirouter.Context) (any, error) {
	return map[string]any{"id": t.ID, "verb": ctx.GetVerb()}, nil
}

type plain struct {
	ID string `json:"id"`
}

func newTestRouter() (*apirouter.Router, *pobj.Object) {
	root := pobj.NewRoot()
	root.RegisterActions("Thing", &pobj.ObjectActions{
		Fetch: pobj.Static(func(ctx context.Context, id string) (*thing, error) {
			if id == "missing" {
				return nil, apirouter.ErrNotFound
			}
			return &thing{ID: id}, nil
		}),
		List: pobj.Static(func(ctx context.Context) ([]string, error) { return []string{"a", "b"}, nil }),
	})
	root.RegisterActions("Plain", &pobj.ObjectActions{
		Fetch: pobj.Static(func(ctx context.Context, id string) (*plain, error) { return &plain{ID: id}, nil }),
	})
	root.RegisterMethod("Thing:echo", func(ctx context.Context) (any, error) {
		v, _ := apirouter.GetParam[string](ctx, "v")
		return v, nil
	})
	root.RegisterMethod("Thing:put", func(ctx context.Context) (any, error) {
		var b []byte
		if body, err := apirouter.GetRequestBody(ctx); err == nil {
			b, _ = io.ReadAll(body)
		}
		var c *apirouter.Context
		ctx.Value(&c)
		return c.GetVerb() + " " + string(b), nil
	}).SetVerbs("PUT", "POST")
	root.RegisterMethod("Thing:raw", func(ctx context.Context) (any, error) {
		var c *apirouter.Context
		ctx.Value(&c)
		c.SetFlag("raw", true)
		c.Header().Set("X-Test", "1")
		c.Header().Set("Cache-Control", "private, no-store")
		v, _ := apirouter.GetParam[string](ctx, "v")
		switch v {
		case "fail":
			return nil, apirouter.ErrForbidden("error_nope", "Nope")
		case "redirect":
			return nil, webutil.RedirectErrorCode(&url.URL{Scheme: "https", Host: "example.com", Path: "/"}, http.StatusFound)
		}
		return v, nil
	})
	root.RegisterMethod("Thing:fail", func(ctx context.Context) (any, error) {
		return nil, apirouter.ErrForbidden("error_nope", "Nope")
	})
	return &apirouter.Router{Root: root}, root
}

func do(t *testing.T, h http.Handler, verb, path string, body io.Reader, hdr map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(verb, path, body)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	var m map[string]any
	json.Unmarshal(rr.Body.Bytes(), &m)
	return rr, m
}

func TestRouterRoot(t *testing.T) {
	rt, _ := newTestRouter()

	rr, m := do(t, rt, "GET", "/Thing", nil, nil)
	if rr.Code != 200 || m["result"] != "success" || len(m["data"].([]any)) != 2 {
		t.Errorf("list: %d %s", rr.Code, rr.Body)
	}
	if m["request_id"] == "" || m["time"] == nil {
		t.Errorf("envelope lacks request_id/time: %s", rr.Body)
	}
	// the global tree does not know Thing
	rr, m = do(t, apirouter.HTTP, "GET", "/Thing", nil, nil)
	if rr.Code != 404 || m["token"] != "error_not_found" {
		t.Errorf("global tree: %d %s", rr.Code, rr.Body)
	}
	// a second router is independent
	other := &apirouter.Router{Root: pobj.NewRoot()}
	if rr, _ := do(t, other, "GET", "/Thing", nil, nil); rr.Code != 404 {
		t.Errorf("other router: %d", rr.Code)
	}
}

func TestObjectHandler(t *testing.T) {
	rt, _ := newTestRouter()
	for _, verb := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		rr, m := do(t, rt, verb, "/Thing/abc", nil, nil)
		data, _ := m["data"].(map[string]any)
		if rr.Code != 200 || data["id"] != "abc" || data["verb"] != verb {
			t.Errorf("%s Thing/abc: %d %s", verb, rr.Code, rr.Body)
		}
	}
	if rr, m := do(t, rt, "GET", "/Thing/missing", nil, nil); rr.Code != 404 || m["token"] != "error_not_found" {
		t.Errorf("missing: %d %s", rr.Code, rr.Body)
	}
	// objects without ApiHandle keep the default behaviour
	if rr, m := do(t, rt, "GET", "/Plain/x", nil, nil); rr.Code != 200 || m["data"].(map[string]any)["id"] != "x" {
		t.Errorf("plain get: %d %s", rr.Code, rr.Body)
	}
	if rr, _ := do(t, rt, "POST", "/Plain/x", nil, nil); rr.Code != 405 {
		t.Errorf("plain post: %d", rr.Code)
	}
}

func TestMethodVerbs(t *testing.T) {
	rt, _ := newTestRouter()
	// default verbs
	if rr, m := do(t, rt, "GET", "/Thing:echo?v=hi", nil, nil); rr.Code != 200 || m["data"] != "hi" {
		t.Errorf("GET echo: %d %s", rr.Code, rr.Body)
	}
	if rr, _ := do(t, rt, "PUT", "/Thing:echo", strings.NewReader("x"), nil); rr.Code != 405 {
		t.Errorf("PUT echo: %d", rr.Code)
	}
	// SetVerbs
	rr, m := do(t, rt, "PUT", "/Thing:put", strings.NewReader("payload"), map[string]string{"Content-Type": "application/octet-stream"})
	if rr.Code != 200 || m["data"] != "PUT payload" {
		t.Errorf("PUT put: %d %s", rr.Code, rr.Body)
	}
	// no content type: the body is not parsed but stays readable
	if rr, m := do(t, rt, "POST", "/Thing:put", strings.NewReader("z"), nil); rr.Code != 200 || m["data"] != "POST z" {
		t.Errorf("POST put: %d %s", rr.Code, rr.Body)
	}
	for _, verb := range []string{"GET", "DELETE"} {
		if rr, _ := do(t, rt, verb, "/Thing:put", nil, nil); rr.Code != 405 {
			t.Errorf("%s put: %d", verb, rr.Code)
		}
	}
	// OPTIONS lists the verbs
	rr, _ = do(t, rt, "OPTIONS", "/Thing:put", nil, nil)
	if rr.Code != 204 || rr.Header().Get("Access-Control-Allow-Methods") != "PUT, POST, OPTIONS" {
		t.Errorf("OPTIONS put: %d %v", rr.Code, rr.Header())
	}
}

func TestEmptyPost(t *testing.T) {
	rt, _ := newTestRouter()
	// no body, no Content-Type, no Content-Length
	if rr, m := do(t, rt, "POST", "/Thing:echo", nil, nil); rr.Code != 200 || m["result"] != "success" {
		t.Errorf("empty POST: %d %s", rr.Code, rr.Body)
	}
	if rr, m := do(t, rt, "POST", "/Thing:echo", strings.NewReader(`{"v":"j"}`), map[string]string{"Content-Type": "application/json"}); rr.Code != 200 || m["data"] != "j" {
		t.Errorf("json POST: %d %s", rr.Code, rr.Body)
	}
}

func TestEnvelopeHooksAndCORS(t *testing.T) {
	rt, _ := newTestRouter()
	rr, _ := do(t, rt, "GET", "/Thing", nil, map[string]string{"Origin": "https://example.com"})
	if rr.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("CORS headers missing: %v", rr.Header())
	}
	rt.DisableCORS = true
	rt.EnvelopeHooks = append(rt.EnvelopeHooks, func(r *apirouter.Response, env map[string]any) {
		if r.Err() != nil {
			env["request"] = env["request_id"]
			delete(env, "request_id")
			delete(env, "data")
			var e *apirouter.Error
			if errors.As(r.Err(), &e) {
				env["exception"] = "Exception\\Token"
			}
		}
	})
	rr, _ = do(t, rt, "GET", "/Thing", nil, map[string]string{"Origin": "https://example.com"})
	if rr.Header().Get("Access-Control-Allow-Origin") != "" || rr.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Errorf("CORS headers with DisableCORS: %v", rr.Header())
	}
	rr, m := do(t, rt, "GET", "/Thing:fail", nil, nil)
	if _, ok := m["data"]; ok || rr.Code != 403 || m["request"] == nil || m["request_id"] != nil || m["exception"] != "Exception\\Token" || m["token"] != "error_nope" || m["code"] != float64(403) {
		t.Errorf("error envelope: %d %s", rr.Code, rr.Body)
	}
}

func TestRouterHooks(t *testing.T) {
	rt, _ := newTestRouter()
	var seen []string
	rt.RequestHooks = append(rt.RequestHooks, func(c *apirouter.Context) error {
		seen = append(seen, "req "+c.GetPath())
		if c.GetPath() == "Thing:echo" && c.GetVerb() == "DELETE" {
			return apirouter.ErrAccessDenied
		}
		return nil
	})
	rt.ResponseHooks = append(rt.ResponseHooks, func(r *apirouter.Response) error {
		seen = append(seen, "res "+r.Result)
		return nil
	})
	do(t, rt, "GET", "/Thing", nil, nil)
	if rr, _ := do(t, rt, "DELETE", "/Thing:echo", nil, nil); rr.Code != 403 {
		t.Errorf("request hook error: %d", rr.Code)
	}
	if strings.Join(seen, ",") != "req Thing,res success,req Thing:echo" {
		t.Errorf("hooks: %v", seen)
	}
	// the global handler does not run the router's hooks
	seen = nil
	do(t, apirouter.HTTP, "GET", "/Thing", nil, nil)
	if len(seen) != 0 {
		t.Errorf("router hooks ran for HTTP: %v", seen)
	}
}

func TestRaw(t *testing.T) {
	rt, _ := newTestRouter()
	rr, _ := do(t, rt, "GET", "/Thing:raw?v=hello", nil, nil)
	if rr.Code != 200 || rr.Body.String() != "hello" || rr.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("raw string: %d %q %v", rr.Code, rr.Body, rr.Header())
	}
	if rr.Header().Get("X-Test") != "1" || rr.Header().Get("Cache-Control") != "private, no-store" {
		t.Errorf("endpoint headers: %v", rr.Header())
	}
	if rr, _ := do(t, rt, "GET", "/Thing:raw", nil, nil); rr.Code != 204 || rr.Body.Len() != 0 {
		t.Errorf("raw empty string: %d %q", rr.Code, rr.Body)
	}
	// errors keep the JSON error envelope in raw mode
	rr, m := do(t, rt, "GET", "/Thing:raw?v=fail", nil, nil)
	if rr.Code != 403 || m["result"] != "error" || m["token"] != "error_nope" || !strings.HasPrefix(rr.Header().Get("Content-Type"), "application/json") {
		t.Errorf("raw error: %d %s", rr.Code, rr.Body)
	}
	// errors that are http.Handlers answer themselves
	rr, _ = do(t, rt, "GET", "/Thing:raw?v=redirect", nil, nil)
	if rr.Code != http.StatusFound || rr.Header().Get("Location") != "https://example.com/" {
		t.Errorf("raw redirect: %d %v", rr.Code, rr.Header())
	}
}

func TestBodies(t *testing.T) {
	root := pobj.NewRoot()
	root.RegisterMethod("B:params", func(ctx context.Context) (any, error) {
		var c *apirouter.Context
		ctx.Value(&c)
		return c.GetParams(), nil
	}).SetVerbs("POST", "DELETE", "GET")
	rt := &apirouter.Router{Root: root}

	// DELETE with a JSON body
	rr, m := do(t, rt, "DELETE", "/B:params?_=%7B%22q%22%3A1%7D", strings.NewReader(`{"b":1}`), map[string]string{"Content-Type": "application/json"})
	if p, _ := m["data"].(map[string]any); rr.Code != 200 || p["b"] == nil || p["q"] != nil {
		t.Errorf("DELETE body: %d %s", rr.Code, rr.Body)
	}
	// DELETE without a body: the query string
	rr, m = do(t, rt, "DELETE", "/B:params?_=%7B%22q%22%3A1%7D", nil, nil)
	if p, _ := m["data"].(map[string]any); rr.Code != 200 || p["q"] == nil {
		t.Errorf("DELETE query: %d %s", rr.Code, rr.Body)
	}

	// multipart: files carry their content type
	body := "--X\r\nContent-Disposition: form-data; name=\"f\"; filename=\"a.txt\"\r\nContent-Type: text/plain\r\n\r\nhello\r\n" +
		"--X\r\nContent-Disposition: form-data; name=\"v\"\r\n\r\nval\r\n--X--\r\n"
	rr, m = do(t, rt, "POST", "/B:params", strings.NewReader(body), map[string]string{"Content-Type": "multipart/form-data; boundary=X"})
	p, _ := m["data"].(map[string]any)
	f, _ := p["f"].(map[string]any)
	if rr.Code != 200 || p["v"] != "val" || f["filename"] != "a.txt" || f["content_type"] != "text/plain" {
		t.Errorf("multipart: %d %s", rr.Code, rr.Body)
	}
}
