package apirouter

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"

	"github.com/KarpelesLab/emitter"
	"github.com/KarpelesLab/pjson"
	"github.com/coder/websocket"
	"github.com/fxamacker/cbor/v2"
)

var (
	wsClients   = make(map[string]*Context)
	wsclientsLk sync.RWMutex
)

// BroadcastWS sends a message to ALL peers connected to the websocket. It should be formatted with
// at least something similar to: map[string]any{"result": "event", "data": ...}
func BroadcastWS(ctx context.Context, data any) error {
	return emitter.Global.Emit(ctx, "broadcast", "*", data)
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
			// determine if we should use binary or text protocol
			typ := c.selectAcceptedType("application/json", "application/cbor")
			// enfore only 1 accept
			c.accept = []string{typ}
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

func (c *Context) wsListen() {
	// listen for messages on the broadcast system
	l := emitter.Global.OnWithCap("broadcast", 32) // some buffer to avoid dropping events
	defer emitter.Global.Off("broadcast", l)

	l2 := make(chan *emitter.Event, 128)

	go func() {
		defer close(l2)
		defer c.wsc.CloseNow()

		for ev := range l {
			select {
			case l2 <- ev:
				// good
			default:
				// l2 is full, drop client for not listening fast enough
				return
			}
		}
	}()

	for {
		select {
		case <-c.Done():
			return
		case ev, ok := <-l2:
			if !ok {
				return
			}

			if len(ev.Args) < 2 {
				continue
			}
			channel, ok := ev.Args[0].(string)
			if !ok {
				continue
			}
			if c.ListensFor(channel) {
				switch c.accept[0] {
				case "application/cbor":
					bin, err := ev.EncodedArg(1, "cbor", cbor.Marshal)
					if err != nil {
						continue
					}
					c.wsc.Write(c, websocket.MessageBinary, bin)
				case "application/json":
					fallthrough
				default:
					str, err := ev.EncodedArg(1, "json", pjson.Marshal)
					if err != nil {
						continue
					}
					c.wsc.Write(c, websocket.MessageText, str)
				}
			}
		}
	}
}

func (c *Context) handleWebsocket() {
	defer c.wsc.CloseNow()
	defer c.releaseWsClient()
	c.registerWsClient()

	var cancel func()
	c.Context, cancel = context.WithCancel(c.Context)
	defer cancel()

	go c.wsListen()

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
		case websocket.MessageBinary:
			// handle as cbor
			var res *Response
			subCtx, err := NewChild(c, dat, "application/cbor")
			if err != nil {
				res = subCtx.errorResponse(err)
			} else {
				subCtx.SetResponseSink(&websocketSink{ctx: subCtx, wsc: c.wsc, cbor: true})
				res, _ = subCtx.Response()
			}
			buf := &bytes.Buffer{}
			enc := cbor.NewEncoder(buf)
			err = enc.Encode(res.getResponseData())
			if err != nil {
				// no really
				c.wsc.Close(websocket.StatusInvalidFramePayloadData, err.Error())
				return
			}
			c.wsc.Write(c, websocket.MessageBinary, buf.Bytes())
		case websocket.MessageText:
			// handle as json
			var res *Response
			subCtx, err := NewChild(c, dat, "application/json")
			if err != nil {
				res = subCtx.errorResponse(err)
			} else {
				subCtx.SetResponseSink(&websocketSink{ctx: subCtx, wsc: c.wsc, cbor: false})
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
