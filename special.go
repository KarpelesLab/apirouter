package apirouter

func (c *Context) CallSpecial() (any, error) {
	p := c.path

	// p starts with a "@"

	switch p {
	default:
		return nil, ErrNotFound
	}
}
