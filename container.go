package diz

import (
	"fmt"

	"go.uber.org/dig"
)

// Container is the staged declaration started by [New]: the dig root —
// built eagerly by New — plus the collected options, applied by
// [Container.Build].
type Container struct {
	root    *dig.Container
	options []Option
}

// New starts a [Container] declaration: it builds the dig root —
// deferring acyclic-graph verification until values are actually
// resolved, so that registration-time errors surface together at the
// first invoke — and collects the options without applying them;
// registration is deferred to [Container.Build], the second stage.
func New(options ...Option) *Container {
	return &Container{
		root:    dig.New(dig.DeferAcyclicVerification()),
		options: options,
	}
}

// Build applies the collected options to the dig root, in order, and
// returns on the first failure — the error prefixed with the failing
// option; the remaining options never run. Dig is lazy: declare
// provides before the invokes that consume them (the natural writing
// order).
//
// Build is one-shot: the options land in the root New built, so a
// second Build or Resolve on the same container re-registers
// everything and fails on duplicate provides.
func (c *Container) Build() error {
	for _, opt := range c.options {
		if err := opt.Apply(c.root); err != nil {
			return fmt.Errorf("%s: %w", opt, err)
		}
	}
	return nil
}

// Resolve appends an extraction of T to the declaration and builds it,
// returning the extracted value. It guarantees the graph is invoked —
// without any invoke, dig's lazy provides never run and the
// declaration stays inert, so a container that only configures is a
// container that does nothing.
//
// Resolve is terminal: it builds (see [Container.Build] for the
// one-shot semantics).
func (c *Container) Resolve[T any]() (T, error) {
	var out T
	c.options = append(c.options, Populate(&out))
	return out, c.Build()
}
