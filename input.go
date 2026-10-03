package apirouter

import (
	"context"

	jsonv1 "encoding/json"
	"encoding/json/v2"
)

// getInputJson returns the request input as json. It is returned as an encoding/json
// RawMessage as this is the type typutil expects for "input_json".
func (c *Context) getInputJson() jsonv1.RawMessage {
	if c.inputJson != nil {
		if len(c.inputJson) == 0 {
			return nil
		}
		return c.inputJson
	}
	if c.params == nil {
		return nil
	}
	buf, err := json.Marshal(c.params)
	if err != nil {
		return nil
	}
	c.inputJson = buf
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
