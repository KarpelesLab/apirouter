package apirouter

type RequestHook func(c *Context) error
type ResponseHook func(r *Response) error

var (
	RequestHooks  []RequestHook
	ResponseHooks []ResponseHook
)

// CSRFHeaderHook is a sample hook for checking a specific middleware header for csrf validation
func CSRFHeaderHook(c *Context) error {
	if c.req != nil && c.req.Header.Get("Sec-Csrf-Token") == "valid" {
		c.SetCsrfValidated(true)
	}
	return nil
}
