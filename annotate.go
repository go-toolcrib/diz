package diz

// AnnotateOption is one annotation [Annotate] applies to every
// [Declaration] it builds: a function writing the definition in
// place. The built-ins — [Name], [Group], [Private], [Alias] — are the
// vocabulary; compose and spread them across call sites for shared
// conventions.
type AnnotateOption func(*Declaration)

// Annotate registers constructors with annotations — the fx.Annotate
// role: it takes the constructors of the bundle, resets everything
// else (the call is the single annotation authority), applies the
// options in order, and returns fresh definitions — the input bundle
// is never modified.
//
//	diz.Annotate(
//		diz.Provide(NewHTTPServer, NewGRPCServer),
//		diz.Alias[transport.Server](),
//		diz.Group("servers"),
//		diz.Private(),
//	)
//
// Name, Group and Private assign — the last option wins; Alias
// accumulates. Compose conventions at the option level (a slice of
// options spread into the call), not by nesting Annotate.
func Annotate(provide ProvideOption, opts ...AnnotateOption) ProvideOption {
	annotated := make(ProvideOption, len(provide))
	for i, def := range provide {
		decl := Declaration{
			provide: def.provide,
		}
		for _, opt := range opts {
			opt(&decl)
		}
		annotated[i] = decl
	}
	return annotated
}

// Name names the results of the annotated definitions.
func Name(name string) AnnotateOption {
	return func(d *Declaration) { d.name = name }
}

// Group adds the results of the annotated definitions to a value group
// (supports ",flatten").
func Group(group string) AnnotateOption {
	return func(d *Declaration) { d.group = group }
}

// Private restricts the annotated definitions to the enclosing [Module]
// and its descendants.
func Private() AnnotateOption {
	return func(d *Declaration) { d.private = true }
}

// Alias also registers the results of the annotated definitions under
// interface type T. Composable across interfaces.
func Alias[T any]() AnnotateOption {
	return func(d *Declaration) { d.alias = append(d.alias, new(T)) }
}
