package apirouter

type RequestHook func(c *Context) error
type ResponseHook func(r *Response) error

var (
	RequestHooks  []RequestHook
	ResponseHooks []ResponseHook
)
