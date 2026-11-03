package diz

import (
	"errors"
	"fmt"
	"strings"

	"go.uber.org/dig"
)

// Provide teaches the container how to build values by registering the
// constructors, in the same spirit as dig.Provide: one or more per
// call.
//
// The constructor follows dig's rules: zero or more parameters, one or
// more results, and an optional trailing error. Dependencies are
// resolved lazily at Invoke time.
//
// Bare registration only — like fx.Provide, annotations live on the
// wrapper: use [Annotate] to name, group, register under interfaces or
// mark private.
func Provide(constructors ...any) ProvideOption {
	set := make(ProvideOption, len(constructors))
	for i, ctor := range constructors {
		set[i] = Declaration{provide: ctor}
	}
	return set
}

// ProvideOption is the Option built by [Provide]: one or more
// declarations.
type ProvideOption []Declaration

func (o ProvideOption) Apply(c Registrar) error {
	var errs error
	for _, d := range o {
		if d.name != "" && d.group != "" {
			errs = errors.Join(errs, errors.New("diz: Provide cannot specify both Name and Group"))
			continue
		}
		var opts []dig.ProvideOption
		if d.name != "" {
			opts = append(opts, dig.Name(d.name))
		}
		if d.group != "" {
			opts = append(opts, dig.Group(d.group))
		}
		if len(d.alias) > 0 {
			opts = append(opts, dig.As(d.alias...))
		}
		// fx.Module semantics: provides are exported to every scope unless
		// Private. At the root this is a no-op.
		opts = append(opts, dig.Export(!d.private))
		if err := c.Provide(d.provide, opts...); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func (o ProvideOption) String() string {
	items := make([]string, len(o))
	for i, d := range o {
		items[i] = d.String()
	}
	return fmt.Sprintf("diz.Provide(%s)", strings.Join(items, "; "))
}
