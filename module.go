package diz

import (
	"errors"
	"fmt"
	"strings"
)

// ModuleOption bundles options into a named scope unit (see
// [Module]). A plain data record.
type ModuleOption struct {
	Name    string
	Options []Option
}

// Module bundles options into a named unit backed by a dig scope.
//
// Visibility follows fx semantics: provides inside a module are exported
// to every scope by default (visible app-wide); mark individual provides
// [Declaration.Private] to keep them visible only to the module and
// its descendants. Decorations and invocations are scoped to the
// module's sub-graph.
//
// Module names are not deduplicated: every call creates a fresh dig
// scope, so two same-named siblings are independent scopes — their
// private provides and decorations do not interact.
//
//	diz.Module("servers",
//		diz.Annotate(
//			diz.Provide(NewHTTPServer),
//			diz.Alias[transport.Server](),
//			diz.Group("servers"),
//			diz.Private(),
//		),
//		diz.Populate(&servers), // collectors live inside
//	)
//
// To apply a convention over a module's options, transform the
// inner list before wrapping — options are plain data records.
func Module(name string, options ...Option) Option {
	return ModuleOption{Name: name, Options: options}
}

func (o ModuleOption) Apply(c Registrar) error {
	// A module is a dig scope: children register into it directly.
	child := c.Scope(o.Name)
	var errs error
	for _, opt := range o.Options {
		if err := opt.Apply(child); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func (o ModuleOption) String() string {
	items := make([]string, len(o.Options))
	for i, opt := range o.Options {
		items[i] = fmt.Sprint(opt)
	}
	return fmt.Sprintf("diz.Module(%q, %s)", o.Name, strings.Join(items, ", "))
}
