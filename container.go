package diz

import (
	"errors"
	"fmt"

	"go.uber.org/dig"
)

// Container is the staged declaration started by [New]: a plain data
// record holding the collected options.
type Container struct {
	options []Option
}

// New starts a [Container] declaration: it collects the options without
// applying them — registration is deferred to [Container.Build], the
// second stage.
func New(options ...Option) *Container {
	return &Container{
		options: options,
	}
}

// Build applies the collected options to a fresh dig container, in
// order, and returns the joined errors — each error prefixed with the
// failing option. Dig is lazy: declare provides before the invokes that
// consume them (the natural writing order).
//
// The underlying dig container defers acyclic-graph verification until
// values are actually resolved, so that registration-time errors
// surface together.
func (c *Container) Build() error {
	root := dig.New(dig.DeferAcyclicVerification())
	var errs error
	for _, opt := range c.options {
		if err := opt.Apply(root); err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", opt, err))
		}
	}
	return errs
}

// Resolve appends an extraction of T to the declaration and builds it,
// returning the extracted value. It guarantees the graph is invoked —
// without any invoke, dig's lazy provides never run and the
// declaration stays inert, so a container that only configures is a
// container that does nothing.
//
// Resolve is terminal: it builds. A later Resolve or Build on the same
// container re-applies every option into a fresh dig container.
func (c *Container) Resolve[T any]() (T, error) {
	var out T
	c.options = append(c.options, Populate(&out))
	return out, c.Build()
}
