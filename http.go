package apirouter

import (
	"net/http"

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
