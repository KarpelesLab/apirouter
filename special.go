package apirouter

import (
	"io/fs"
)

func (c *Context) CallSpecial() (any, error) {
	p := c.path

	// p starts with a "@"

	switch p {
	default:
		return nil, fs.ErrNotExist
	}
}
