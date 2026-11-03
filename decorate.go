package diz

import (
	"fmt"
)

// Decorate replaces how a value is built: the decorator receives the
// currently-provided value of the type it accepts and returns the
// replacement, in the same spirit as dig.Decorate: one decorator per
// call.
//
// Decorators must be registered before the values they decorate are
// resolved, so register them before any [Invoke] that consumes them.
func Decorate(decorator any) DecorateOption {
	return DecorateOption{Decorator: decorator}
}

// DecorateOption is the Option built by [Decorate]: it registers
// a decorator on the container. A plain data record.
type DecorateOption struct {
	Decorator any
}

func (o DecorateOption) Apply(c Registrar) error {
	return c.Decorate(o.Decorator)
}

func (o DecorateOption) String() string {
	return fmt.Sprintf("diz.Decorate(%T)", o.Decorator)
}
