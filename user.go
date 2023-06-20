package apirouter

import "context"

// GetUser will return the user object if it matches type T. If there is no
// user object or it is not of the right type, nil will be returned
func GetUser[T any](ctx context.Context) *T {
	v, ok := ctx.Value("user_object").(*T)
	if ok {
		return v
	}
	return nil
}
