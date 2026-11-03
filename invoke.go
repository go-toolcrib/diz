package diz

import (
	"fmt"
)

// Invoke runs the function immediately, resolving its dependencies from
// the container, in the same spirit as dig.Invoke: one function per
// call.
//
// Invoke is intended for side effects (wiring servers, registering
// hooks). Errors are reported through the Build result.
func Invoke(invoke any) InvokeOption {
	return InvokeOption{Invoke: invoke}
}

// InvokeOption is the Option built by [Invoke]: it runs its
// function immediately against the container. A plain data record.
type InvokeOption struct {
	Invoke any
}

func (o InvokeOption) Apply(c Registrar) error {
	return c.Invoke(o.Invoke)
}

func (o InvokeOption) String() string {
	return fmt.Sprintf("diz.Invoke(%T)", o.Invoke)
}
