package diz

import (
	"fmt"
	"strings"
)

// Declaration declares a single provide: one constructor with its
// annotations — the Spring BeanDefinition / fx.Annotated role.
// Encapsulated metadata: declarations are built only by [Provide] and
// [Annotate]; the getters expose them for inspection and rewriting
// conventions.
type Declaration struct {
	provide any
	name    string
	group   string // supports ",flatten"
	alias   []any  // interface markers, e.g. new(http.Handler)
	private bool   // visible only to the enclosing Module and its descendants
}

// Provide returns the constructor.
func (d Declaration) Provide() any { return d.provide }

// Name returns the name annotation, "" when unset.
func (d Declaration) Name() string { return d.name }

// Group returns the group annotation, "" when unset (may carry
// dig's ",flatten" suffix).
func (d Declaration) Group() string { return d.group }

// Alias returns the interface markers registered via [Alias].
func (d Declaration) Alias() []any { return d.alias }

// Private reports whether visibility is restricted to the enclosing
// [Module] and its descendants.
func (d Declaration) Private() bool { return d.private }

func (d Declaration) String() string {
	var s strings.Builder
	s.WriteString(fmt.Sprintf("diz.Provide(%T)", d.provide))
	if d.name != "" {
		s.WriteString(fmt.Sprintf(", diz.Name(%q)", d.name))
	}
	if d.group != "" {
		s.WriteString(fmt.Sprintf(", diz.Group(%q)", d.group))
	}
	for _, marker := range d.alias {
		s.WriteString(fmt.Sprintf(", diz.Alias[%T]()", marker))
	}
	if d.private {
		s.WriteString(", diz.Private()")
	}
	return s.String()
}
