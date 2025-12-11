package apirouter

// RequestHook is a function type for intercepting requests before they are processed.
// Hooks can be used for authentication, authorization, logging, or request modification.
// Return an error to abort the request and return an error response to the client.
type RequestHook func(c *Context) error

// ResponseHook is a function type for intercepting responses before they are sent.
// Hooks can be used for logging, response modification, or adding extra data.
// Return an error to replace the response with an error response.
type ResponseHook func(r *Response) error

var (
	// RequestHooks is a slice of hooks that will be executed before each request.
	// Hooks are executed in order; if any hook returns an error, subsequent hooks
	// are skipped and an error response is returned.
	RequestHooks []RequestHook

	// ResponseHooks is a slice of hooks that will be executed after generating a response.
	// Hooks are executed in order for all responses including error responses.
	ResponseHooks []ResponseHook
)

// CSRFHeaderHook is a sample hook for checking a specific middleware header for CSRF validation.
// It checks for the "Sec-Csrf-Token" header with value "valid" and marks the request as CSRF-validated.
// This is provided as an example; production applications should implement proper CSRF token validation.
func CSRFHeaderHook(c *Context) error {
	if c.req != nil && c.req.Header.Get("Sec-Csrf-Token") == "valid" {
		c.SetCsrfValidated(true)
	}
	return nil
}
