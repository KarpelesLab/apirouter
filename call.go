package apirouter

import (
	"net/http"
	"strings"

	"github.com/KarpelesLab/pobj"
	"github.com/KarpelesLab/webutil"
)

func (c *Context) Call() (any, error) {
	p := c.path
	if len(p) >= 1 && p[0] == '@' {
		return c.CallSpecial()
	}

	r := pobj.Root()
	m := ""
	method := false
	corsReq := c.verb == "OPTIONS"
	var obj any

	if pos := strings.LastIndexByte(p, ':'); pos != -1 {
		m = p[pos+1:]
		p = p[:pos]
		method = true
	}

	ps := strings.Split(p, "/")

	for _, s := range ps {
		// detect what is "s"
		if len(s) == 0 {
			// return error?
			continue
		}

		if s[0] >= 'A' && s[0] <= 'Z' {
			// starts with A-Z: this is likely a class name
			v := r.Child(s)
			if v != nil {
				r = v
				obj = nil
				continue
			}
			//return nil, ErrNotFound
		}

		// this is an attempt to load as ID
		if obj != nil {
			// you can't have Object/id/id_again
			return nil, ErrNotFound
		}
		// make sure we have a GET action
		if r.Action == nil {
			return nil, ErrNotFound
		}
		get := r.Action.Fetch
		if get == nil {
			return nil, ErrNotFound
		}
		if corsReq {
			// ignore loading the actual ID if in cors req
			obj = true
			continue
		}

		res, err := get.CallArg(c, struct{ Id string }{Id: s})
		if err != nil {
			return nil, err
		}

		c.objects[r.String()] = res
		obj = res
	}

	// ok we need to return a class
	if method {
		if corsReq {
			c.flags["raw"] = true
			return nil, &optionsResponder{[]string{"GET", "POST", "HEAD", "OPTIONS"}}
		}
		// ok we need to call a static method
		meth := r.Static(m)
		if meth == nil {
			return nil, ErrNotFound
		}
		switch c.verb {
		case "HEAD", "GET", "POST":
			return meth.CallArg(c, c.params)
		default:
			return nil, webutil.HttpError(http.StatusMethodNotAllowed)
		}
	}

	if obj != nil {
		if corsReq {
			c.flags["raw"] = true
			return nil, &optionsResponder{[]string{"GET", "HEAD", "OPTIONS", "PATCH", "DELETE"}}
		}
		switch c.verb {
		case "HEAD", "GET": // Fetch (default)
			return obj, nil
		case "PATCH": // Update
			if res, ok := obj.(Updatable); ok {
				err := res.ApiUpdate(c)
				if err != nil {
					return nil, err
				}
				return obj, nil
			}
			return nil, webutil.HttpError(http.StatusMethodNotAllowed)
		case "DELETE": // Delete
			if res, ok := obj.(Deletable); ok {
				err := res.ApiDelete(c)
				if err != nil {
					return nil, err
				}
				return obj, nil
			}
			return nil, webutil.HttpError(http.StatusMethodNotAllowed)
		default:
			return nil, webutil.HttpError(http.StatusMethodNotAllowed)
		}
	}

	// need to call list
	if r.Action == nil {
		return nil, ErrNotFound
	}

	if corsReq {
		c.flags["raw"] = true
		return nil, &optionsResponder{[]string{"GET", "HEAD", "OPTIONS", "POST", "DELETE"}}
	}

	switch c.verb {
	case "HEAD", "GET": // List
		if list := r.Action.List; list != nil {
			return list.CallArg(c, c.params)
		}
		return nil, webutil.HttpError(http.StatusMethodNotAllowed)
	case "POST": // Create
		if create := r.Action.Create; create != nil {
			return create.CallArg(c, c.params)
		}
		return nil, webutil.HttpError(http.StatusMethodNotAllowed)
	case "DELETE": // Clear
		if clear := r.Action.Clear; clear != nil {
			return clear.CallArg(c, c.params)
		}
		return nil, webutil.HttpError(http.StatusMethodNotAllowed)
	default:
		return nil, webutil.HttpError(http.StatusMethodNotAllowed)
	}
}
