package apirouter

// Updatable is an interface that objects can implement to support PATCH requests.
// When a PATCH request is made to an object endpoint, ApiUpdate will be called
// with the request context, allowing the object to update itself based on the
// request parameters.
type Updatable interface {
	ApiUpdate(ctx *Context) error
}

// Deletable is an interface that objects can implement to support DELETE requests.
// When a DELETE request is made to an object endpoint, ApiDelete will be called
// with the request context, allowing the object to handle its own deletion.
type Deletable interface {
	ApiDelete(ctx *Context) error
}
