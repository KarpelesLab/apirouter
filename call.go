package apirouter

import (
	"io/fs"
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
			//return nil, fs.ErrNotExist
		}

		// attempt to load as ID
		if r.Action == nil {
			return nil, fs.ErrNotExist
		}
		get := r.Action.Fetch
		if get == nil {
			return nil, fs.ErrNotExist
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
		// ok we need to call a static method
		meth := r.Static(m)
		if meth == nil {
			return nil, fs.ErrNotExist
		}
		return meth.Call(c)
	}

	if obj != nil {
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
		return nil, fs.ErrNotExist
	}

	switch c.verb {
	case "HEAD", "GET": // List
		if list := r.Action.List; list != nil {
			return list.CallArg(c, nil)
		}
		return nil, webutil.HttpError(http.StatusMethodNotAllowed)
	case "POST": // Create
		if create := r.Action.Create; create != nil {
			return create.CallArg(c, nil)
		}
		return nil, webutil.HttpError(http.StatusMethodNotAllowed)
	case "DELETE": // Clear
		if clear := r.Action.Clear; clear != nil {
			return clear.CallArg(c, nil)
		}
		return nil, webutil.HttpError(http.StatusMethodNotAllowed)
	default:
		return nil, webutil.HttpError(http.StatusMethodNotAllowed)
	}
}
