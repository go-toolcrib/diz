package diz

import (
	"errors"
	"fmt"
)

// Populate fills target with a value resolved from the container, in the
// same spirit as fx.Populate.
//
// To extract several values at once, point at a struct that embeds
// [In]; dig unwraps it and resolves every field:
//
//	var got target
//	diz.Populate(&got)
//
// where target is:
//
//	type target struct {
//		diz.In
//
//		N int
//		S string
//	}
func Populate[T any](target *T) PopulateOption[T] {
	return PopulateOption[T]{Target: target}
}

// PopulateOption is the Option built by [Populate]: it writes one
// resolved value into its target. A plain data record.
type PopulateOption[T any] struct {
	Target *T
}

func (o PopulateOption[T]) Apply(c Registrar) error {
	if o.Target == nil {
		return errors.New("diz: Populate target must be a non-nil pointer")
	}
	// Populate is sugar over Invoke: a concrete closure — no special path.
	return c.Invoke(func(v T) { *o.Target = v })
}

func (o PopulateOption[T]) String() string {
	return fmt.Sprintf("diz.Populate(%T)", o.Target)
}
