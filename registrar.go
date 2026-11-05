// Package diz wraps go.uber.org/dig with an fx-flavored option
// API and modern Go (generics, literal data records, an open option
// interface). It is a thin wrapper, not the next fx: no lifecycle, no
// module-tree runners — options register straight into dig.
//
// diz is for assembling dependencies at startup: declare the graph,
// Build once during initialization, and let it run. It is not a
// runtime container — nothing registers after Build, and it does not
// serve dynamic use once the application has started.
//
// Typical usage:
//
//	if err := diz.New(
//		diz.Provide(NewHTTPServer),
//		diz.Provide(NewGRPCServer),
//		diz.Supply(config),
//		diz.Invoke(bootstrap),
//	).Build(); err != nil {
//		panic(err)
//	}
//
// The package requires Go 1.27 or later.
package diz

import (
	"go.uber.org/dig"
)

// Registrar is the registration surface a [Option] applies to: the exact
// method set *dig.Container and *dig.Scope share, so the dig root and
// module sub-scopes are interchangeable targets. fx keeps the same
// interface private, as the target of fx.Module.
type Registrar interface {
	Provide(constructor any, opts ...dig.ProvideOption) error
	Invoke(function any, opts ...dig.InvokeOption) error
	Decorate(decorator any, opts ...dig.DecorateOption) error
	Scope(name string, opts ...dig.ScopeOption) *dig.Scope
}

// Compile-time proof both dig targets satisfy Registrar.
var (
	_ Registrar = (*dig.Container)(nil)
	_ Registrar = (*dig.Scope)(nil)
)

// Option instructs the registration surface: it is applied by
// [Container.Build], in order, against the root [Registrar]; a [Module]
// executes its children against a nested scope.
//
// The interface is deliberately open: implement Apply on your own types
// to package arbitrary configuration logic — parameterized registration,
// conditional wiring, loops over discovered components — and pass them
// to [New] alongside the built-in options:
//
//	type ServerGroup struct {
//		Servers []transport.Server
//	}
//
//	func (g *ServerGroup) Apply(c diz.Registrar) error {
//		for _, server := range g.Servers {
//			if err := diz.Annotate(
//				diz.Provide(func() transport.Server { return server }),
//				diz.Group("servers"),
//			).Apply(c); err != nil {
//				return err
//			}
//		}
//		return nil
//	}
//
//	if err := diz.New(
//		serverGroup,
//		diz.Provide(NewConfig),
//		diz.Invoke(bootstrap),
//	).Build(); err != nil {
//		panic(err)
//	}
type Option interface {
	Apply(Registrar) error
}
