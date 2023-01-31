package apirouter

type Updatable interface {
	ApiUpdate(ctx *Context) error
}

type Deletable interface {
	ApiDelete(ctx *Context) error
}
