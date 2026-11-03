package diz

// Replace replaces the value of T wherever it is resolved, in the same
// spirit as fx.Replace: a zero-parameter decorator — the original value
// is never built, so a broken dependency chain behind the original
// provider does not surface.
//
// Like Supply, the static type T is pinned at the call site; interface
// values replace the interface type (fx.Replace instead reflects on the
// dynamic type). The original type must still be provided somewhere —
// a decorator wraps a provider, it is not one.
func Replace[T any](v T) Option {
	return Decorate(func() T { return v })
}
