# diz

[![Go 1.27+](https://img.shields.io/badge/Go-1.27%2B-00ADD8)](https://go.dev)

[English](README.md) | 简体中文

基于 [go.uber.org/dig](https://go.uber.org/dig) 的选项式依赖注入容器。

diz 以 fx 风格的声明式 API 描述依赖注册，以纯数据记录承载选项，
以泛型弥补 dig 与 fx 在类型推导上的不足。它不引入 fx 的运行时机制：
没有生命周期管理，没有 Start/Stop 钩子，没有模块树装配器——每条选项
在 `Build` 时直接翻译为 dig 注册。

## 亮点

- **选项即数据**——选项是可检视、可改写的记录：大多数字段全导出（可字面量
  构造、可用普通代码改写过滤生成）；注册元数据（`Declaration`）封装在
  构造器与 getter 之后，生而良构。fx 的 option 是不透明闭包，这里的数据可检视。
- **泛型优先**——`Supply[T]`、`Populate[T]` 按静态类型注册与提取；
  `Alias[T]()` 是泛型函数，向接口注册无需
  `new(http.Handler)` 式标记值，类型正确性由编译器保证。
- **统一的注册表面**——选项面向 `Registrar` 接口注册，它是
  `*dig.Container` 与 `*dig.Scope` 的共享方法集：根容器与模块子域
  可互换，与 fx 的内部设计同形。
- **无生命周期**——没有 Start/Stop 钩子、没有后台 goroutine、没有装配器状态，
  选项在 `Build` 时直接翻译为 dig 注册。
- **错误一次报告**——所有选项的错误按 `errors.Join` 聚合，随
  `Build` 一次性返回。

## 环境要求

- Go 1.27 及以上——`(*Container).Resolve[T]()` 用到了泛型方法。

## 目录

- [快速上手](#快速上手)
- [选项](#选项)
- [注解](#注解)
- [模块与可见性](#模块与可见性)
- [泛型 API](#泛型-api)
- [自定义选项](#自定义选项)
- [执行模型](#执行模型)

## 快速上手

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

- 两段式：`New` 收集选项；`Build` 创建新的 dig 容器并按序应用，错误聚合后
  返回（每条错误带选项前缀）。
- 不配置任何 invoke 则 provide 全部不执行：以 `Invoke`、`Populate` 收尾，
  或用终结方法 `(*Container).Resolve[T]()` 提取值并强制图执行。
- dig 惰性解析依赖：provide 需声明在消费它的 invoke 之前。

## 选项

| 构造器 | 字面量 | 用途 |
|---|---|---|
| `Provide(constructors ...)` | —（封装；见 `Declaration`） | 注册构造器（可一次多个） |
| `Annotate(provide, opts ...AnnotateOption)` | —（带注解的注册路径） | 将注解应用到每个声明 |
| `Declaration` | —（封装；getter 方法 `Provide`/`Name`/`Group`/`Alias`/`Private`） | 注册记录——仅由 `Provide`/`Annotate` 生产 |
| `Name(name)` | —（`Annotate` 内的兄弟选项） | 为被注解的 Declaration 命名 |
| `Group(group)` | —（`Annotate` 内的兄弟选项） | 将被注解的 Declaration 加入值组 |
| `Private()` | —（`Annotate` 内的兄弟选项） | 将可见性收缩到所在模块 |
| `Alias[T]()` | —（`Annotate` 内的兄弟选项） | 同时注册到接口 `T` |
| `Invoke(function)` | `InvokeOption{Invoke}` | 执行副作用函数 |
| `Decorate(decorator)` | `DecorateOption{Decorator}` | 替换值的构建方式 |
| `Supply[T](v)` | —（`Provide` 的泛型糖） | 注册现成的值 |
| `Replace[T](v)` | —（`Decorate` 的泛型糖） | 替换已解析的值（不构建原值） |
| `Populate[T](&target)` | `PopulateOption[T]{Target}` | 提取解析出的值 |
| `(*Container).Resolve[T]()` | —（终结操作：追加一条 `Populate` 后构建） | 提取值并强制图被解析执行 |
| `Module(name, opts ...)` | `ModuleOption{Name, Options}` | 命名模块（dig scope） |
| `Options(opts ...)` | `OptionGroup{...}` | 打包一组选项 |

`Provide` 每次可收一个或多个构造器；注解兄弟（`Name`、`Group`、
`Private`、`Alias`）只在 `Annotate` 内有意义——编译器会拒绝在其中传入
其他选项。可字面量构造的记录，字段名即元素名（`Invoke`、`Decorator`）。

## 注解

```go
diz.Annotate(
    diz.Provide(NewHTTPServer, NewGRPCServer),
    diz.Alias[transport.Server](),
    diz.Group("servers"),
    diz.Private(),
)
```

Annotate 只取集合中的构造器、其余字段重置（本次调用就是注解的唯一入口），再把
注解选项按序作用到全新的声明上——输入不变，结果恰好就是本次调用声明的内容。
`Name`、`Group`、`Private` 是赋值语义——后写的生效；`Alias` 是叠加语义。
每个 `AnnotateOption` 都是 `func(*Declaration)`，公共约定可以收集成
切片后展开复用：

```go
defaults := []diz.AnnotateOption{diz.Group("servers"), diz.Private()}
diz.Annotate(diz.Provide(NewHTTP), defaults...)
```

声明是封装的：只由 `Provide` 和 `Annotate` 生产，任何人可通过 getter 方法
（`Provide`、`Name`、`Group`、`Alias`、`Private`）读取——检视与
改写约定可行，私自捏造不行。

| 兄弟选项 | 说明 |
|---|---|
| `Name` | 注册为具名值，与 `Group` 互斥 |
| `Group` | 加入值组，支持 dig 的 `",flatten"` 后缀 |
| `Alias[T]` | 同时注册到接口 `T`，可跨接口组合 |
| `Private` | 将可见性收缩到所在模块及其后代 |

注意：仅 `Alias` 不做分组。向同一接口注册两次会得到 duplicate-definition
错误；收集一个接口的多个实现需要 `Alias` 与 `Group` 组合，消费端以带
`group:"..."` 标签的 `[]T` 接收。进入值组的产物不能被裸类型注入，只有
组切片可以消费——这为按接口收集实现提供了隔离保证。

上表的兄弟选项即全部注解词汇；目前没有原生 `dig.ProvideOption` 的逃生舱。

## 模块与可见性

```go
diz.Module("servers",
    diz.Annotate(
        diz.Provide(NewHTTP),
        diz.Private(),
    ),
    diz.Populate(&servers), // 在模块内完成收集
)
```

Module 对应一个 dig scope，子选项直接注册到其中。可见性遵循 fx.Module
语义：

- provide 默认全局可见：模块内注册的 provide 对根容器与兄弟模块可见
  （内部实现为 `dig.Export(true)`）。
- `Private` 将可见性收缩到本模块及其后代。
- Decorate 与 Invoke 没有导出概念，只作用于本模块的子图。
- 模块可嵌套，祖先的注册对后代可见。
- 模块名不查重：每次 `Module` 调用都会新建一个 dig scope，同名兄弟模块
  相互独立——各自的 Private provide 与 Decorate 互不影响。

## 泛型 API

```go
// Supply：将值注册为静态类型 T
var h http.Handler = handler
diz.Supply(h)              // 注册为 http.Handler
diz.Supply[*mux.Router](h) // 显式指定具体类型

// Populate：提取单个解析值，或指向 In 结构体一次提取多个
var got target
diz.Populate(&got)
```

`In` 与 `Out` 是 dig 参数对象与结果对象的别名：`In` 结构体在一个参数
位置声明多个依赖，`Out` 结构体让单个构造器产出多个值。

## 自定义选项

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

`(*Container).Build` 以根容器为 `c` 逐条调用 `Apply`；`Module` 以子 scope 为 `c`
调用其子选项。实现 `Option` 即可封装任意注册逻辑：

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

`Registrar` 各方法的 `opts` 变参接受原生 dig 选项。

## 执行模型

`New(opts...)` 收集选项；`Build` 新建 dig 容器并按序应用。

`Resolve[T]()` 是终结形态：追加一条 `Populate` 后构建，返回提取的值——
这就是"图一定会执行"的保证。`Build` 与 `Resolve` 都会把全部选项重新应用到
全新的 dig 容器，重复调用会重跑所有副作用。

除数据记录本身外没有中间表示：选项是数据记录，`Build` 将其直接翻译为
dig 注册，错误聚合进返回值。dig 保持惰性——依赖在 Invoke 时才解析，
注册期错误会汇合在同一次返回中。
