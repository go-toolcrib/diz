package diz

import (
	"go.uber.org/dig"
)

// In marks a struct as a dig parameter object. Embed it in a struct type to
// consume several dependencies as one parameter:
//
//	type Input struct {
//		diz.In
//
//		Logger *slog.Logger
//		Config *Config
//	}
type In = dig.In

// Out marks a struct as a dig result object. Embed it in a struct type to
// produce several dependencies from one constructor:
//
//	type Output struct {
//		diz.Out
//
//		Logger *slog.Logger
//		Config *Config
//	}
type Out = dig.Out
