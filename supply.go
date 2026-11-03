package diz

// Supply provides an instantiated value to the container as if it had been
// provided by a constructor that simply returns it, in the same spirit as
// fx.Supply.
//
// The value is registered under its static type T, inferred at the call
// site. Unlike fx.Supply, which reflects on the dynamic type, this means
// interface values are registered as the interface type:
//
//	var h http.Handler = handler
//	diz.Supply(h) // provides http.Handler
//
// To register such a value under its concrete type instead, pin T
// explicitly:
//
//	diz.Supply[*mux.Router](h)
func Supply[T any](v T) Option {
	return Provide(func() T { return v })
}
