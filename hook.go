package apirouter

type RequestHook func(c *Context)
type ResponseHook func(r *Response)

var (
	RequestHooks  []RequestHook
	ResponseHooks []ResponseHook
)
