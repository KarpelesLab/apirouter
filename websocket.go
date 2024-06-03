package apirouter

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/KarpelesLab/pjson"
	"nhooyr.io/websocket"
)

func (c *Context) prepareWebsocket() (any, error) {
	var opts *websocket.AcceptOptions
	if c.csrfOk {
		// csrf token is valid, so we accept any host
		opts = &websocket.AcceptOptions{InsecureSkipVerify: true}
	}

	// return a *Response for websocket upgrade
	res := &Response{
		Result: "upgrade",
		Code:   101,
		ctx:    c,
		subhandler: func(rw http.ResponseWriter, req *http.Request) {
			wsc, err := websocket.Accept(rw, req, opts)
			if err != nil {
				// in this case, we already have a response sent to the client
				return
			}
			handleWebsocket(c, wsc)
		},
	}

	return res, nil
}

func handleWebsocket(c *Context, wsc *websocket.Conn) {
	defer wsc.CloseNow()

	wsc.SetReadLimit(128 * 1024)
	ctx := context.Background()

	for {
		mt, dat, err := wsc.Read(ctx)
		if err != nil {
			if err == io.EOF {
				return
			}
			// slog.Debug?
			return
		}

		switch mt {
		case websocket.MessageText:
			// handle as json
			var res *Response
			subCtx, err := NewChild(c, dat)
			if err != nil {
				res = subCtx.errorResponse(time.Now(), err)
			} else {
				res, _ = subCtx.Response()
			}
			buf := &bytes.Buffer{}
			enc := pjson.NewEncoderContext(res.getJsonCtx(), buf)
			err = enc.Encode(res.getResponseData())
			if err != nil {
				// no really
				wsc.Close(websocket.StatusInvalidFramePayloadData, err.Error())
				return
			}
			wsc.Write(ctx, websocket.MessageText, buf.Bytes())
		default:
		}
	}
}
