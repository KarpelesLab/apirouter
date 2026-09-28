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

// ObjectHandler is an interface that objects returned by a Fetch action can
// implement to handle the requests made on them ("Object/id") with any
// verb: GET, HEAD, POST, PUT, PATCH and DELETE all call ApiHandle, whose
// result is returned as the response (the verb is available through
// ctx.GetVerb()). It takes precedence over Updatable and Deletable, and
// over returning the object itself on GET.
type ObjectHandler interface {
	ApiHandle(ctx *Context) (any, error)
}
