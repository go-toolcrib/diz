# diz

[![Go 1.27+](https://img.shields.io/badge/Go-1.27%2B-00ADD8)](https://go.dev)

English | [简体中文](README.zh-CN.md)

An option-based dependency injection container built on
[go.uber.org/dig](https://go.uber.org/dig).

diz describes dependency registration with an fx-flavored declarative
API, carries each option as a plain data record, and uses generics where
dig and fx fall short on type inference. It does not adopt fx's runtime:
no lifecycle management, no Start/Stop hooks, no module-tree runners —
every option translates directly into dig registrations at `Build`.

## Highlights

- **Options are data** — options are records you can inspect and rewrite:
  most carry exported fields (construct as literals, transform with plain
  code); registration metadata (`Declaration`) is encapsulated behind
  constructors and getter methods — well-formed by construction. fx options
  are opaque closures; here the data is inspectable.
- **Generics first** — `Supply[T]` and `Populate[T]` register and extract
  by static type; `Alias[T]()` is a generic function that registers
  under an interface without marker values like
  `new(http.Handler)` — type correctness is checked by the compiler.
- **One registration surface** — options apply to the `Registrar`
  interface, the shared method set of `*dig.Container` and `*dig.Scope`:
  the root container and module sub-scopes are interchangeable, mirroring
  fx's internal design.
- **No lifecycle** — no Start/Stop hooks, no background goroutines, no
  assembler state; options translate directly into dig registrations at
  `Build`.
- **Errors reported once** — errors from all options are joined with
  `errors.Join` and returned by `Build` in one shot.

## Requirements

- Go 1.27 or later — `(*Container).Resolve[T]()` uses method type
  parameters.

## Contents

- [Quick start](#quick-start)
- [Options](#options)
- [Annotations](#annotations)
- [Modules and visibility](#modules-and-visibility)
- [Generic API](#generic-api)
- [Custom options](#custom-options)
- [Execution model](#execution-model)

## Quick start

```go
if err := diz.New(
    diz.Provide(NewHTTPServer),
    diz.Provide(NewGRPCServer),
    diz.Supply(config),
    diz.Invoke(bootstrap),
).Build(); err != nil {
    panic(err)
}
```

- Two phases: `New` collects the options; `Build` creates a fresh dig
  container and applies them in order, returning the joined errors —
  each prefixed with the failing option.
- Without an invoke nothing runs: end the declaration with `Invoke`,
  `Populate`, or the terminal `(*Container).Resolve[T]()`, which extracts
  a value and forces the graph to execute.
- dig resolves lazily: declare provides before the invokes that consume
  them.

## Options

| Constructor | Literal | Purpose |
|---|---|---|
| `Provide(constructors ...)` | — (encapsulated; see `Declaration`) | Register constructors |
| `Annotate(provide, opts ...AnnotateOption)` | — (the annotated registration path) | Apply annotations to every declaration |
| `Declaration` | — (encapsulated; getter methods `Provide`/`Name`/`Group`/`Alias`/`Private`) | The registration record — built only by `Provide`/`Annotate` |
| `Name(name)` | — (sibling inside `Annotate`) | Name the annotated providers |
| `Group(group)` | — (sibling inside `Annotate`) | Add the annotated providers to a value group |
| `Private()` | — (sibling inside `Annotate`) | Restrict visibility to the enclosing module |
| `Alias[T]()` | — (sibling inside `Annotate`) | Also register under interface `T` |
| `Invoke(function)` | `InvokeOption{Invoke}` | Run a side-effect function |
| `Decorate(decorator)` | `DecorateOption{Decorator}` | Replace how a value is built |
| `Supply[T](v)` | — (generic sugar over `Provide`) | Provide ready-made values |
| `Replace[T](v)` | — (generic sugar over `Decorate`) | Replace a resolved value (never builds the original) |
| `Populate[T](&target)` | `PopulateOption[T]{Target}` | Extract resolved values |
| `(*Container).Resolve[T]()` | — (terminal: appends a `Populate`, then builds) | Extract a value and force the graph to run |
| `Module(name, opts ...)` | `ModuleOption{Name, Options}` | Named module (dig scope) |
| `Options(opts ...)` | `OptionGroup{...}` | Bundle options |

`Provide` takes one or more constructors per call; the annotation
siblings (`Name`, `Group`, `Private`, `Alias`) only make sense inside
`Annotate` — the compiler rejects anything else there. The
literal-constructible records name their field after the element
(`Invoke`, `Decorator`).

## Annotations

```go
diz.Annotate(
    diz.Provide(NewHTTPServer, NewGRPCServer),
    diz.Alias[transport.Server](),
    diz.Group("servers"),
    diz.Private(),
)
```

Annotate takes the constructors of the bundle, resets everything else (the
call is the single annotation authority), and applies the options, in order,
to fresh declarations — the input stays unchanged and the result carries
exactly what the call declares. `Name`, `Group` and `Private` assign — the
last option wins; `Alias` accumulates. Every `AnnotateOption` is a
`func(*Declaration)`, so shared conventions can be collected once and
spread:

```go
defaults := []diz.AnnotateOption{diz.Group("servers"), diz.Private()}
diz.Annotate(diz.Provide(NewHTTP), defaults...)
```

Declarations are encapsulated: built only by `Provide` and `Annotate`, readable
by anyone through their getter methods (`Provide`, `Name`, `Group`, `Alias`,
`Private`) — inspection and rewriting conventions work; private
minting does not.

| Sibling | Description |
|---|---|
| `Name` | Registers as a named value; mutually exclusive with `Group` |
| `Group` | Adds to a value group; supports dig's `",flatten"` suffix |
| `Alias[T]` | Also registers under interface `T`; composable across interfaces |
| `Private` | Restricts visibility to the enclosing module and its descendants |

Note: `Alias` alone does not group. Registering the same interface twice
is a duplicate-definition error; collecting multiple implementations of
an interface requires `Alias` combined with `Group`, consumed as a `[]T`
field tagged `group:"..."` inside an `In` struct. Values in a group
cannot be injected by bare type — only the group slice can consume
them — which is the isolation boundary when collecting implementations
by interface.

The sibling table above is the full annotation vocabulary; there is
currently no escape hatch for raw `dig.ProvideOption` values.

## Modules and visibility

```go
diz.Module("servers",
    diz.Annotate(
        diz.Provide(NewHTTP),
        diz.Private(),
    ),
    diz.Populate(&servers), // collect inside the module
)
```

A Module maps to a dig scope; its child options register into it
directly. Visibility follows fx.Module semantics:

- Provides are visible globally by default: a provide registered inside a
  module is visible to the root container and sibling modules
  (implemented as `dig.Export(true)`).
- `Private` restricts visibility to the module and its descendants.
- Decorate and Invoke have no export concept: they affect only the
  module's sub-graph.
- Modules nest; ancestor registrations are visible to descendants.
- Module names are not deduplicated: each `Module` call creates a fresh
  dig scope, so two same-named siblings are independent — their private
  provides and decorations do not interact.

## Generic API

```go
// Supply: register a value under its static type T
var h http.Handler = handler
diz.Supply(h)              // registered as http.Handler
diz.Supply[*mux.Router](h) // pin the concrete type explicitly

// Populate: extract one resolved value, or point at an In struct to
// extract several at once
var got target
diz.Populate(&got)
```

`In` and `Out` alias dig's parameter and result objects: an `In` struct
declares multiple dependencies in one parameter position, and an `Out`
struct lets a single constructor produce multiple values.

## Custom options

```go
type Option interface {
    Apply(c Registrar) error
}

type Registrar interface {
    Provide(constructor any, opts ...dig.ProvideOption) error
    Invoke(function any, opts ...dig.InvokeOption) error
    Decorate(decorator any, opts ...dig.DecorateOption) error
    Scope(name string, opts ...dig.ScopeOption) *dig.Scope
}
```

`(*Container).Build` calls `Apply` with the root container as `c`; `Module` calls
its children with a sub-scope as `c`. Implement `Option` to package
any registration logic:

```go
type ServerGroup struct {
    Servers []transport.Server
}

func (g *ServerGroup) Apply(c diz.Registrar) error {
    for _, server := range g.Servers {
        if err := diz.Annotate(
            diz.Provide(func() transport.Server { return server }),
            diz.Group("servers"),
        ).Apply(c); err != nil {
            return err
        }
    }
    return nil
}
```

The `opts` variadic parameters accept raw dig options.

## Execution model

`New(opts...)` collects the options; `Build` applies them to a fresh
dig container, in order.

`Resolve[T]()` is the terminal form: it appends a `Populate` and builds,
returning the extracted value — the guarantee that the graph runs.
`Build` and `Resolve` re-apply every option into a fresh dig container,
so a second call re-runs every side effect.

There is no intermediate representation beyond the records themselves:
options are data records, `Build` translates them directly into dig
registrations, and errors are joined into the return value. dig stays
lazy — dependencies resolve at Invoke time, so registration-time errors
surface together in one report.
