package apirouter

import (
	"bytes"
	"context"

	"github.com/KarpelesLab/pjson"
)

func (c *Context) getInputJson() pjson.RawMessage {
	if c.inputJson != nil {
		if len(c.inputJson) == 0 {
			return nil
		}
		return c.inputJson
	}
	if c.params == nil {
		return nil
	}
	buf := &bytes.Buffer{}
	enc := pjson.NewEncoderContext(c, buf)
	err := enc.Encode(c.params)
	if err != nil {
		return nil
	}
	c.inputJson = buf.Bytes()
	if len(c.inputJson) == 0 {
		return nil
	}
	return c.inputJson
}

// GetInputJSON returns the raw JSON input for the current request.
// The type parameter T must be a byte slice type (e.g., []byte or json.RawMessage).
// Returns nil if no context is available or if there is no input data.
func GetInputJSON[T ~[]byte](ctx context.Context) T {
	var c *Context
	ctx.Value(&c)
	if c == nil {
		return nil
	}
	return T(c.getInputJson())
}
