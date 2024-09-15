package apirouter

import (
	"context"
	"net/http"
)

type ctxControl int

const ctxGetPreObjects ctxControl = 1

type ctxController struct {
	context.Context
	objects map[string]any
}

// WithValue is a method that can be used to add a value to the context of all requests
// going through a handler. This will automatically call [context.WithValue] on all incoming
// requests.
func WithValue(h http.Handler, key, val any) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		ctx = context.WithValue(ctx, key, val)
		req = req.WithContext(ctx)

		h.ServeHTTP(rw, req)
	}
}

// WithObject can be used to add an object to the generated context, that can then be obtained
// via apirouter.GetObject(ctx, ...).
func WithObject(h http.Handler, typ string, obj any) http.HandlerFunc {
	objs := map[string]any{typ: obj}

	return func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		ctx = &ctxController{Context: ctx, objects: objs}
		req = req.WithContext(ctx)

		h.ServeHTTP(rw, req)
	}
}

func (c *ctxController) Value(key any) any {
	if ctrl, ok := key.(ctxControl); ok {
		switch ctrl {
		case ctxGetPreObjects:
			// fetch from parent (will return a new map if nothing)
			res := getPreObjects(c.Context)
			// copy rather than return so c.objects cannot be modified
			for k, v := range c.objects {
				res[k] = v
			}
			return res
		}
	}

	return c.Context.Value(key)
}

func getPreObjects(ctx context.Context) map[string]any {
	if res, ok := ctx.Value(ctxGetPreObjects).(map[string]any); ok {
		return res
	}
	return make(map[string]any)
}
