package diz

import (
	"errors"
	"fmt"
	"strings"
)

// Options bundles a group of options into a single option.
//
// This is useful for grouping related options together, for example:
//
//	var logging = diz.Options(
//		diz.Provide(NewLogger),
//		diz.Supply(defaultLevel),
//	)
func Options(options ...Option) Option {
	return OptionGroup(options)
}

// OptionGroup applies its options in order and joins their errors.
// Built by [Options].
type OptionGroup []Option

func (o OptionGroup) Apply(c Registrar) error {
	var errs error
	for _, opt := range o {
		if err := opt.Apply(c); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func (o OptionGroup) String() string {
	items := make([]string, len(o))
	for i, opt := range o {
		items[i] = fmt.Sprint(opt)
	}
	return fmt.Sprintf("diz.Options(%s)", strings.Join(items, ", "))
}
