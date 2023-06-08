package apirouter

import (
	"net/http"
	"strings"

	"github.com/KarpelesLab/webutil"
)

// apirouter.HTTP can be used as a handler function, or as a handler
// via http.HandlerFunc(apirouter.HTTP)
func HTTP(rw http.ResponseWriter, req *http.Request) {
	ctx, err := NewHttp(rw, req)
	if err != nil {
		webutil.ErrorToHttpHandler(err).ServeHTTP(rw, req)
		return
	}
	res, _ := ctx.Response()
	res.ServeHTTP(rw, req)
}

type optionsResponder struct {
	allowedMethods []string
}

func (o *optionsResponder) Error() string {
	return "Options responder"
}

func (o *optionsResponder) getAllowedMethods() string {
	if o.allowedMethods == nil {
		return "POST, GET, OPTIONS, PUT, DELETE, PATCH"
	}
	return strings.Join(o.allowedMethods, ", ")
}

func (o *optionsResponder) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// set headers, return no body
	rw.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	rw.Header().Set("Access-Control-Max-Age", "86400")
	rw.Header().Set("Access-Control-Allow-Methods", o.getAllowedMethods())
	rw.WriteHeader(http.StatusNoContent)
}
