package apirouter

import (
	"bytes"
	"context"

	"github.com/KarpelesLab/pjson"
	"github.com/coder/websocket"
	"github.com/fxamacker/cbor/v2"
)

// EncoderInterface defines the interface for encoding response data.
// Any encoder that implements Encode(obj any) error can be used as a response sink.
type EncoderInterface interface {
	Encode(obj any) error
}

// EncoderSink wraps an encoder implementing EncoderInterface as a ResponseSink.
// This allows using any compatible encoder (such as json.Encoder or cbor.Encoder)
// to receive API responses.
func EncoderSink(enc EncoderInterface) ResponseSink {
	return &encoderSink{enc: enc}
}

type encoderSink struct {
	enc EncoderInterface
}

func (e *encoderSink) SendResponse(r *Response) error {
	return e.enc.Encode(r.getResponseData())
}

type websocketSink struct {
	ctx  context.Context
	wsc  *websocket.Conn
	cbor bool
}

func (w *websocketSink) SendResponse(r *Response) error {
	if w.cbor {
		buf := &bytes.Buffer{}
		enc := cbor.NewEncoder(buf)
		err := enc.Encode(r.getResponseData())
		if err != nil {
			return err
		}
		return w.wsc.Write(w.ctx, websocket.MessageBinary, buf.Bytes())
	} else {
		buf := &bytes.Buffer{}
		enc := pjson.NewEncoderContext(r.getJsonCtx(), buf)
		err := enc.Encode(r.getResponseData())
		if err != nil {
			return err
		}
		return w.wsc.Write(w.ctx, websocket.MessageText, buf.Bytes())
	}
}
