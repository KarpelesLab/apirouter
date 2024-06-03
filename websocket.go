package apirouter

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/KarpelesLab/pjson"
	"nhooyr.io/websocket"
)

var (
	wsClients   = make(map[string]*Context)
	wsclientsLk sync.RWMutex
)

// BroadcastWS sends a message to ALL peers connected to the websocket. It should be formatted with
// at least something similar to: map[string]any{"result": "event", "data": ...}
func BroadcastWS(ctx context.Context, data any) error {
	str, err := pjson.MarshalContext(ctx, data)
	if err != nil {
		return err
	}

	clients := listWsClients()
	for _, c := range clients {
		if wsc := c.wsc; wsc != nil {
			wsc.Write(ctx, websocket.MessageText, str)
		}
	}
	return nil
}

func listWsClients() []*Context {
	wsclientsLk.RLock()
	defer wsclientsLk.RUnlock()

	res := make([]*Context, 0, len(wsClients))
	for _, c := range wsClients {
		res = append(res, c)
	}
	return res
}

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
			// switch rw to wsc
			c.rw = nil
			c.wsc = wsc
			// run handler loop
			c.handleWebsocket()
		},
	}

	return res, nil
}

func (c *Context) registerWsClient() {
	wsclientsLk.Lock()
	defer wsclientsLk.Unlock()

	wsClients[c.reqid] = c
}

func (c *Context) releaseWsClient() {
	wsclientsLk.Lock()
	defer wsclientsLk.Unlock()

	delete(wsClients, c.reqid)
}

func (c *Context) handleWebsocket() {
	defer c.wsc.CloseNow()
	defer c.releaseWsClient()
	c.registerWsClient()

	c.wsc.SetReadLimit(128 * 1024)

	for {
		mt, dat, err := c.wsc.Read(c)
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
				c.wsc.Close(websocket.StatusInvalidFramePayloadData, err.Error())
				return
			}
			c.wsc.Write(c, websocket.MessageText, buf.Bytes())
		default:
		}
	}
}
