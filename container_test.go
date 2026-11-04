package diz_test

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/go-toolcrib/diz"
)

// build is one-shot sugar over the two-phase API (New + Build), so
// tests can declare options and apply them in a single expression.
func build(options ...diz.Option) error {
	return diz.New(options...).Build()
}

func assertSameElements(t *testing.T, got []string, want ...string) {
	t.Helper()
	sorted := append([]string(nil), got...)
	slices.Sort(sorted)
	sortedWant := append([]string(nil), want...)
	slices.Sort(sortedWant)
	if !slices.Equal(sorted, sortedWant) {
		t.Fatalf("got %v, want %v", sorted, sortedWant)
	}
}

type greeter interface {
	Greet() string
}

type englishGreeter struct{}

func (englishGreeter) Greet() string { return "hello" }
func (englishGreeter) Write() string { return "text" }

// TestLiteralConstruction: registration metadata (ProvideOption and its
// definitions) is encapsulated — produced only by the API — while the
// remaining options stay plain data records; literal construction
// works end to end for them.
func TestLiteralConstruction(t *testing.T) {
	type deps struct {
		diz.In

		Greeters []greeter `group:"lit"`
	}
	var greetings []string
	var populated int

	err := build(
		diz.OptionGroup{
			diz.Provide(func() int { return 7 }),
			diz.Annotate(
				diz.Provide(
					func() englishGreeter { return englishGreeter{} },
					func() germanGreeter { return germanGreeter{} },
				),
				diz.Alias[greeter](),
				diz.Group("lit"),
			),
		},
		diz.InvokeOption{Invoke: func(d deps) {
			for _, g := range d.Greeters {
				greetings = append(greetings, g.Greet())
			}
		}},
		diz.ModuleOption{
			Name: "inner",
			Options: []diz.Option{
				diz.DecorateOption{Decorator: func(n int) int { return n + 1 }},
				diz.PopulateOption[int]{Target: &populated},
			},
		},
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	assertSameElements(t, greetings, "hello", "hallo")
	if populated != 8 { // 7 经模块内 Decorate +1
		t.Fatalf("populated %d, want 8 (decorated)", populated)
	}
}

// TestPreBuildRewrite: options are plain data records — a convention
// (autoServers style) transforms the slice before handing it to New.
func TestPreBuildRewrite(t *testing.T) {
	type deps struct {
		diz.In

		Servers []greeter `group:"app:servers"`
	}
	var greetings []string

	options := []diz.Option{
		diz.Provide(func() englishGreeter { return englishGreeter{} }),
		diz.Provide(func() germanGreeter { return germanGreeter{} }),
		diz.Provide(func() int { return 1 }), // 无关 provider 透传
		diz.Invoke(func(d deps) {
			for _, g := range d.Servers {
				greetings = append(greetings, g.Greet())
			}
		}),
	}
	greeterType := reflect.TypeFor[greeter]()
	for i, d := range options {
		set, ok := d.(diz.ProvideOption)
		if !ok {
			continue
		}
		annotated := false
		for _, p := range set {
			t := reflect.TypeOf(p.Provide())
			if t == nil {
				continue
			}
			for j := range t.NumOut() {
				annotated = annotated || t.Out(j).Implements(greeterType)
			}
		}
		if annotated {
			options[i] = diz.Annotate(set, diz.Alias[greeter](), diz.Group("app:servers"))
		}
	}

	if err := build(options...); err != nil {
		t.Fatalf("Build: %v", err)
	}
	assertSameElements(t, greetings, "hello", "hallo")
}

// TestGroupedResultsAreNotBareInjectable: dig's group registration hides
// results from bare-type resolution — only the group slice can consume
// them. This is the isolation boundary behind grouped server providers.
func TestGroupedResultsAreNotBareInjectable(t *testing.T) {
	grouped := diz.Annotate(
		diz.Provide(func() englishGreeter { return englishGreeter{} }),
		diz.Alias[greeter](),
		diz.Group("sealed"),
	)

	type deps struct {
		diz.In

		List []greeter `group:"sealed"`
	}
	if err := build(grouped, diz.Invoke(func(d deps) { _ = d.List })); err != nil {
		t.Fatalf("group consumption should work: %v", err)
	}

	if err := build(grouped, diz.Invoke(func(greeter) {})); err == nil {
		t.Fatal("bare interface injection should fail on grouped results")
	}
	if err := build(grouped, diz.Invoke(func(englishGreeter) {})); err == nil {
		t.Fatal("bare concrete-type injection should fail on grouped results")
	}
}

// TestScopeExportsProvidesByDefault: fx.Module semantics — module
// provides are exported to every scope (dig.Export), so the root sees
// them without any extra wiring.
func TestScopeExportsProvidesByDefault(t *testing.T) {
	var got string
	err := build(
		diz.Module("feature",
			diz.Provide(func() string { return "exported" }),
		),
		diz.Invoke(func(s string) { got = s }),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got != "exported" {
		t.Fatalf("got %q, want the module provide visible at the root", got)
	}
}

// TestScopePrivateProvidesStayInside: Private provides are visible to
// the module (and its descendants) and invisible everywhere else.
func TestScopePrivateProvidesStayInside(t *testing.T) {
	private := diz.Annotate(
		diz.Provide(func() englishGreeter { return englishGreeter{} }),
		diz.Private(),
	)

	// 模块内可见
	if err := build(
		diz.Module("servers", private, diz.Invoke(func(englishGreeter) {})),
	); err != nil {
		t.Fatalf("inside the module the private provide is visible: %v", err)
	}

	// 根不可见
	if err := build(
		diz.Module("servers", private),
		diz.Invoke(func(englishGreeter) {}),
	); err == nil {
		t.Fatal("private provides must be invisible outside the module")
	}
}

// TestScopeSeesRootProviders: module scopes see ancestor providers —
// root registrations propagate down.
func TestScopeSeesRootProviders(t *testing.T) {
	var got int
	err := build(
		diz.Provide(func() int { return 7 }),
		diz.Module("child",
			diz.Invoke(func(n int) { got = n }),
		),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got != 7 {
		t.Fatalf("got %d, want the root provider visible inside the module", got)
	}
}

// TestAnnotateName: the sibling options set annotations on the wrapped
// provides — the fx.Annotate role over plain data records.
func TestAnnotateName(t *testing.T) {
	type deps struct {
		diz.In

		Greeting string `name:"ro"`
	}
	var got string
	err := build(
		diz.Annotate(
			diz.Provide(func() string { return "hello" }),
			diz.Name("ro"),
		),
		diz.Invoke(func(d deps) { got = d.Greeting }),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}

// TestAnnotateLeavesInputUnchanged: Annotate works on a copy — a shared
// base bundle can be annotated differently per consumer without
// cross-contamination.
func TestAnnotateLeavesInputUnchanged(t *testing.T) {
	base := diz.Provide(func() int { return 1 })

	diz.Annotate(base, diz.Group("g"), diz.Private())

	if base[0].Group() != "" || base[0].Private() {
		t.Fatalf("Annotate mutated its input: %+v", base[0])
	}
}

// TestAnnotateStartsFromBare: Annotate keeps only the constructors of
// its input — pre-set annotations are reset (fx.Annotate semantics:
// the call is the single annotation authority), and the input bundle
// is never modified.
func TestAnnotateStartsFromBare(t *testing.T) {
	base := diz.Annotate(diz.Provide(func() int { return 1 }), diz.Name("ro"))

	staged := diz.Annotate(base, diz.Private())

	if d := staged[0]; d.Name() != "" || !d.Private() {
		t.Fatalf("want bare definition with Private, got %v", d)
	}
	if d := base[0]; d.Name() != "ro" {
		t.Fatalf("input mutated: %v", d)
	}
}

// TestAnnotateAppliesToAll: one annotation set over a variadic Provide —
// the autoServers convention made first-class.
func TestAnnotateAppliesToAll(t *testing.T) {
	type deps struct {
		diz.In

		Greeters []greeter `group:"greeters"`
	}
	var greetings []string
	err := build(
		diz.Annotate(
			diz.Provide(
				func() englishGreeter { return englishGreeter{} },
				func() germanGreeter { return germanGreeter{} },
			),
			diz.Alias[greeter](),
			diz.Group("greeters"),
		),
		diz.Invoke(func(d deps) {
			for _, g := range d.Greeters {
				greetings = append(greetings, g.Greet())
			}
		}),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	assertSameElements(t, greetings, "hello", "hallo")
}

// TestAnnotatePrivate: the sibling reaches Private (module-scoped visibility).
func TestAnnotatePrivate(t *testing.T) {
	inside := diz.Annotate(
		diz.Provide(func() englishGreeter { return englishGreeter{} }),
		diz.Private(),
	)

	if err := build(
		diz.Module("servers", inside, diz.Invoke(func(englishGreeter) {})),
	); err != nil {
		t.Fatalf("inside the module the private provide is visible: %v", err)
	}
	if err := build(
		diz.Module("servers", inside),
		diz.Invoke(func(englishGreeter) {}),
	); err == nil {
		t.Fatal("private provides must be invisible outside the module")
	}
}

func TestSupplyRegistersValue(t *testing.T) {
	err := build(
		diz.Supply(42),
		diz.Supply("answer"),
		diz.Invoke(func(n int, s string) {
			if n != 42 || s != "answer" {
				t.Fatalf("got %d %q", n, s)
			}
		}),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
}

func TestSupplyInterfaceValueRegisteredAsInterface(t *testing.T) {
	var g greeter = englishGreeter{}
	err := build(
		diz.Supply(g),
		diz.Invoke(func(got greeter) {
			if got.Greet() != "hello" {
				t.Fatalf("got %q", got.Greet())
			}
		}),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
}

func TestPopulateFillsTarget(t *testing.T) {
	var got int
	err := build(
		diz.Provide(func() int { return 7 }),
		diz.Populate(&got),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got != 7 {
		t.Fatalf("Populate filled %d, want 7", got)
	}
}

// TestPopulateInStructFillsAllFields: standalone variadic Provide
// registers every constructor in one call.
func TestPopulateInStructFillsAllFields(t *testing.T) {
	type target struct {
		diz.In

		N int
		S string
	}
	var got target
	err := build(
		diz.Provide(
			func() int { return 1 },
			func() string { return "two" },
		),
		diz.Populate(&got),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.N != 1 || got.S != "two" {
		t.Fatalf("Populate filled %+v", got)
	}
}

func TestPopulateNilTargetReportsErr(t *testing.T) {
	var p *int
	if err := build(diz.Populate(p)); err == nil {
		t.Fatal("Populate(nil) must surface an error, not panic later")
	}
}

func TestContainerResolveExtractsValue(t *testing.T) {
	b := diz.New(
		diz.Provide(func() int { return 42 }),
	)
	got, err := b.Resolve[int]()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != 42 {
		t.Fatalf("Resolve got %d, want 42", got)
	}

	_, err = diz.New().Resolve[int]()
	if err == nil {
		t.Fatal("Resolve of an unprovided type must report the missing dependency")
	}
}

func TestDecorateReplacesDependency(t *testing.T) {
	err := build(
		diz.Provide(func() int { return 40 }),
		diz.Decorate(func(n int) int { return n + 2 }),
		diz.Invoke(func(n int) {
			if n != 42 {
				t.Fatalf("decorated dependency got %d, want 42", n)
			}
		}),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
}

type germanGreeter struct{}

func (germanGreeter) Greet() string { return "hallo" }

// Alias does NOT auto-group: registering the same interface twice via Alias is a
// duplicate-definition error. Collecting implementations requires combining
// Alias with Group (see TestProvideAliasWithGroupCollectsImplementations).
func TestProvideSameAsTwiceRejected(t *testing.T) {
	newAs := func(ctor any) diz.ProvideOption {
		return diz.Annotate(diz.Provide(ctor), diz.Alias[greeter]())
	}
	err := build(
		newAs(func() englishGreeter { return englishGreeter{} }),
		newAs(func() germanGreeter { return germanGreeter{} }),
	)
	if err == nil || !strings.Contains(err.Error(), "already provided") {
		t.Fatalf("Build() = %v, want duplicate registration rejection", err)
	}
}

// TestProvideAliasWithGroupCollectsImplementations: combining Alias with Group is
// the manual path for collecting several implementations of one interface
// into a slice via an In-struct group tag.
func TestProvideAliasWithGroupCollectsImplementations(t *testing.T) {
	type deps struct {
		diz.In

		Greeters []greeter `group:"greeters"`
	}
	var greetings []string
	err := build(
		diz.Annotate(
			diz.Provide(func() englishGreeter { return englishGreeter{} }),
			diz.Alias[greeter](),
			diz.Group("greeters"),
		),
		diz.Annotate(
			diz.Provide(func() germanGreeter { return germanGreeter{} }),
			diz.Alias[greeter](),
			diz.Group("greeters"),
		),
		diz.Invoke(func(d deps) {
			for _, g := range d.Greeters {
				greetings = append(greetings, g.Greet())
			}
		}),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	assertSameElements(t, greetings, "hello", "hallo")
}

func TestProvideName(t *testing.T) {
	type deps struct {
		diz.In

		Greeting string `name:"greeting"`
	}
	var got string
	err := build(
		diz.Annotate(
			diz.Provide(func() string { return "hello" }),
			diz.Name("greeting"),
		),
		diz.Provide(func() int { return 1 }), // 注解与裸构造器混用
		diz.Invoke(func(d deps) { got = d.Greeting }),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestProvideGroupFlatten(t *testing.T) {
	type deps struct {
		diz.In

		List []string `group:"g"`
	}
	var got []string
	err := build(
		diz.Annotate(
			diz.Provide(func() []string { return []string{"a", "b"} }),
			diz.Group("g,flatten"),
		),
		diz.Annotate(
			diz.Provide(func() string { return "c" }),
			diz.Group("g"),
		),
		diz.Invoke(func(d deps) { got = d.List }),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	assertSameElements(t, got, "a", "b", "c")
}

func TestProvideAlias(t *testing.T) {
	type speaker interface{ Greet() string }
	type writer interface{ Write() string }

	err := build(
		diz.Annotate(
			diz.Provide(func() englishGreeter { return englishGreeter{} }),
			diz.Alias[greeter](),
			diz.Alias[speaker](),
			diz.Alias[writer](),
		),
		diz.Invoke(func(g greeter, s speaker, w writer) {
			if g.Greet() != "hello" || s.Greet() != "hello" || w.Write() != "text" {
				t.Fatal("Alias markers did not register under all interfaces")
			}
		}),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
}

func TestProvideAliasWithName(t *testing.T) {
	type deps struct {
		diz.In

		G greeter `name:"multi"`
	}
	var got greeter
	err := build(
		diz.Annotate(
			diz.Provide(func() englishGreeter { return englishGreeter{} }),
			diz.Alias[greeter](),
			diz.Name("multi"),
		),
		diz.Invoke(func(d deps) { got = d.G }),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got == nil || got.Greet() != "hello" {
		t.Fatalf("Alias+Name combo got %v", got)
	}
}

func TestProvideNameAndGroupRejected(t *testing.T) {
	err := build(
		diz.Annotate(
			diz.Provide(func() string { return "v" }),
			diz.Name("x"),
			diz.Group("y"),
		),
	)
	if err == nil || !strings.Contains(err.Error(), "both Name and Group") {
		t.Fatalf("Build() = %v, want Name/Group mutual-exclusion rejection", err)
	}
}

func TestAnnotateString(t *testing.T) {
	opt := diz.Annotate(
		diz.Provide(func() string { return "hello" }),
		diz.Name("greeting"),
	)
	if s := fmt.Sprint(opt); !strings.Contains(s, `diz.Name("greeting")`) {
		t.Errorf("String() = %q", s)
	}
}

func TestOptionStrings(t *testing.T) {
	for name, option := range map[string]diz.Option{
		"Provide":  diz.Provide(func() int { return 1 }),
		"Invoke":   diz.Invoke(func() {}),
		"Decorate": diz.Decorate(func(n int) int { return n }),
		"Options":  diz.Options(diz.Provide(func() int { return 1 })),
		"Supply":   diz.Supply(1),
		"Populate": diz.Populate(new(int)),
	} {
		if s := fmt.Sprint(option); !strings.Contains(s, "diz.") || s == "" {
			t.Errorf("%s.String() = %q", name, s)
		}
	}
}

// TestCustomOption proves the Option interface is implementable outside the
// diz package: define a type, implement Apply, pass it to New
// directly alongside the built-in options.
type ServerGroup struct {
	Servers []string
}

func (g *ServerGroup) Apply(s diz.Registrar) error {
	for _, srv := range g.Servers {
		if err := diz.Annotate(
			diz.Provide(func() string { return srv }),
			diz.Group("servers"),
		).Apply(s); err != nil {
			return err
		}
	}
	return nil
}

var _ diz.Option = (*ServerGroup)(nil)

func TestCustomOptionDynamicallyConfiguresDig(t *testing.T) {
	type deps struct {
		diz.In

		List []string `group:"servers"`
	}
	var got []string
	err := build(
		&ServerGroup{Servers: []string{"http", "grpc"}},
		diz.Provide(func() int { return 1 }),
		diz.Invoke(func(d deps) { got = d.List }),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("server group produced %v, want 2 entries", got)
	}
	assertSameElements(t, got, "http", "grpc")
}

func TestCustomOptionErrorSurfacesViaBuild(t *testing.T) {
	err := build(errString("boom"))
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Build() = %v, want boom", err)
	}
}

type errString string

func (e errString) Apply(diz.Registrar) error { return errors.New(string(e)) }

var _ diz.Option = errString("")

type funcOption func(diz.Registrar) error

func (f funcOption) Apply(s diz.Registrar) error { return f(s) }

var _ diz.Option = funcOption(nil)

func TestBuildProvideInvokeChain(t *testing.T) {
	type dep struct{ greeting string }

	var invoked bool
	err := build(
		diz.Provide(func() dep { return dep{greeting: "hello"} }),
		diz.Invoke(func(d dep) {
			invoked = true
			if d.greeting != "hello" {
				t.Fatalf("got %q", d.greeting)
			}
		}),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !invoked {
		t.Fatal("Invoke did not run")
	}
}

// TestBuildFailsFast: Build stops at the first failing option — the
// error is prefixed with the failing option, and later options never
// run.
func TestBuildFailsFast(t *testing.T) {
	var invoked bool
	err := build(
		diz.Provide(func() string { return "ok" }),
		diz.Provide("not a constructor"),
		diz.Invoke(func() { invoked = true }),
	)
	if err == nil || !strings.Contains(err.Error(), "must provide constructor function") {
		t.Fatalf("Build() = %v, want the bad constructor reported", err)
	}
	if !strings.Contains(err.Error(), "diz.Provide(string)") {
		t.Errorf("Build() should name the failing option, got: %v", err)
	}
	if invoked {
		t.Fatal("options after the first failure must not run")
	}
}

// TestBuildIsOneShot: options land in the root New built — a second
// Build re-registers them and fails on duplicate provides.
func TestBuildIsOneShot(t *testing.T) {
	c := diz.New(diz.Provide(func() int { return 1 }))
	if err := c.Build(); err != nil {
		t.Fatalf("first Build: %v", err)
	}
	if err := c.Build(); err == nil {
		t.Fatal("second Build must fail on duplicate registrations")
	}
}

func TestOptionsGroupsAndPropagatesErrors(t *testing.T) {
	err := build(
		diz.Options(
			diz.Provide(func() int { return 1 }),
			diz.Provide(42),
		),
	)
	if err == nil {
		t.Fatal("error inside Options() group was swallowed")
	}
	if !strings.Contains(err.Error(), "diz.Options(") {
		t.Errorf("Build() should name the failing option, got: %v", err)
	}
}

func TestReplaceOverridesSuppliedValue(t *testing.T) {
	type cfg struct{ Addr string }
	var got cfg
	err := build(
		diz.Supply(cfg{Addr: ":8080"}),
		diz.Replace(cfg{Addr: ":9090"}),
		diz.Populate(&got),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Addr != ":9090" {
		t.Fatalf("got %+v, want the replaced value", got)
	}
}

// TestReplaceDoesNotBuildOriginal: Replace registers a zero-parameter
// decorator — the original provider is never invoked, so its own missing
// dependencies never surface. (A prev-taking decorator would fail here.)
func TestReplaceDoesNotBuildOriginal(t *testing.T) {
	type cfg struct{ Addr string }
	type dep struct{}
	var got cfg
	err := build(
		diz.Provide(func(dep) cfg { return cfg{} }), // dep 未提供
		diz.Replace(cfg{Addr: ":7070"}),
		diz.Populate(&got),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Addr != ":7070" {
		t.Fatalf("got %+v, want the replaced value", got)
	}
}
