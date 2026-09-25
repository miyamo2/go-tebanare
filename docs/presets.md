# Presets

<!-- Generated from internal/presets by "go test -run TestPresetsDocUpToDate -update .". Do not edit. -->

Presets are built-in rules. A configuration enables a preset by listing its name under `presets`:

```yaml
version: 1
presets:
  - getter
  - iferr
  - noop
```

To change the settings of a preset, write its name as a key and the settings as the value:

```yaml
presets:
  - getter:
      max_depth: 1
```

Settings that the configuration leaves out keep their defaults. The criteria describe each preset with the default settings, and the examples under "With these settings" show what a setting changes.

| Preset | Hides | Summary |
|---|---|---|
| [`getter`](#getter) | function declarations | Methods that only return a field of the receiver. |
| [`iferr`](#iferr) | statements | `if err != nil` blocks that return the error unchanged. |
| [`noop`](#noop) | function declarations | Methods that take no parameters, return nothing, and have an empty body. |

## getter

Methods that only return a field of the receiver. Hides: function declarations.

### Criteria

With the default settings, code matches when all of these hold:

- The function is a method with a named receiver. Value, pointer, and generic receiver types all count.
- It takes no parameters.
- It returns exactly one value of any type. A named result is allowed.
- The body is one return statement that returns a field of the receiver, such as `u.name`, or a chain of fields, such as `u.cfg.timeout`. Parentheses are ignored.
- The first field after the receiver (`x` in `u.x`) is not the name of a method that the same file declares on the same type, since `u.x` would then be a method value.
- No comment other than the doc comment is on the lines of the declaration.

### Settings

| Setting | Type | Default | Description |
|---|---|---|---|
| `paths` | []string | none | Globs that limit the files this preset applies to, on top of `files.include` and `files.exclude`. |
| `exclude_paths` | []string | none | Globs of files this preset skips, on top of `files.exclude`. |
| `include_doc` | bool | true | Hide the doc comment together with the function. |
| `max_depth` | int | unlimited | The maximum number of fields in the returned chain: with 1, `u.name` matches and `u.cfg.timeout` does not. |

### Examples

With the default settings:

Matches.

```go
func (u *User) Name() string { return u.name }
```

Matches. A named result and parentheses are allowed.

```go
func (u User) ID() (id int64) { return (u.id) }
```

Matches. Generic receiver types are allowed.

```go
func (s *Stack[T]) Len() int { return s.n }
```

Matches. A chain of fields is allowed, and the doc comment does not affect the result.

```go
// Timeout returns the request timeout.
func (u *User) Timeout() time.Duration { return u.cfg.timeout }
```

Does not match. It does more than return a field.

```go
func (u *User) Title() string { return strings.TrimSpace(u.title) }
```

Does not match. It takes a parameter.

```go
func (u *User) NameOr(def string) string { return u.name }
```

Does not match. It returns two values.

```go
func (u *User) Pair() (string, int) { return u.name, u.age }
```

Does not match. It indexes a field.

```go
func (u *User) First() string { return u.items[0] }
```

Does not match. It calls a method.

```go
func (u *User) Limit() int { return u.cfg.Limit() }
```

Does not match. `kind` is not a field of the receiver.

```go
func (*User) Kind() string { return kind }
```

Does not match. It has a comment.

```go
func (u *User) Age() int { return u.age /* TODO */ }
```

With these settings:

```yaml
max_depth: 1
```

Matches. One field is within `max_depth`.

```go
func (u *User) Name() string { return u.name }
```

Does not match. The chain has two fields, more than `max_depth`.

```go
func (u *User) Timeout() time.Duration { return u.cfg.timeout }
```

## iferr

`if err != nil` blocks that return the error unchanged. Hides: statements.

### Criteria

With the default settings, code matches when all of these hold:

- The condition is `err != nil` and nothing else. Parentheses are ignored. `nil != err` does not match.
- The body is one return statement, and its last result is the identifier from the condition. Parentheses are ignored. A wrapped error, such as `fmt.Errorf("load: %w", err)` or `errors.Wrap(err, "load")`, does not match.
- The statement has no init statement (`if err := f(); ...`) and no else branch.
- No comment is on the hidden lines, including a comment after the closing brace.
- The other results hold no function call (except the builtin `new`), no function literal, and no channel receive. A bare return does not match.

### Settings

| Setting | Type | Default | Description |
|---|---|---|---|
| `paths` | []string | none | Globs that limit the files this preset applies to, on top of `files.include` and `files.exclude`. |
| `exclude_paths` | []string | none | Globs of files this preset skips, on top of `files.exclude`. |
| `names` | []string | [err] | Names of error variables, as globs matched against the whole name, where `*` matches any run of characters and `?` matches one. |
| `allow_comments` | bool | false | Hide the statement even when a comment is on the hidden lines. |
| `init` | string (exclude, fold-body) | exclude | How to treat if statements with an init statement: `exclude` keeps them visible; `fold-body` hides the lines from `return` to the closing brace and keeps the header line visible. |
| `allow_bare_return` | bool | false | Also hide a bare return. |
| `allow_calls_in_results` | bool | false | Allow function calls in the results before the error; function literals and channel receives still do not match. |

### Examples

With the default settings:

Matches.

```go
if err != nil {
	return err
}
```

Matches.

```go
if err != nil {
	return nil, err
}
```

Matches.

```go
if err != nil {
	return *new(T), err
}
```

Does not match. It wraps the error.

```go
if err != nil {
	return fmt.Errorf("load config: %w", err)
}
```

Does not match. Another statement runs before `return`.

```go
if err != nil {
	log.Printf("load: %v", err)
	return err
}
```

Does not match. It has an init statement.

```go
if err := load(); err != nil {
	return err
}
```

Does not match. It uses a bare return.

```go
if err != nil {
	return
}
```

Does not match. It has a comment.

```go
if err != nil { // the caller logs it
	return err
}
```

With these settings:

```yaml
names: [err, "*Err"]
```

Matches. `"*Err"` in `names` adds `parseErr`.

```go
if parseErr != nil {
	return nil, parseErr
}
```

Does not match. The condition and the return use different variables.

```go
if fooErr != nil {
	return nil, barErr
}
```

With these settings:

```yaml
allow_comments: true
```

Matches. `allow_comments: true` hides statements with comments.

```go
if err != nil { // the caller logs it
	return err
}
```

With these settings:

```yaml
init: fold-body
```

Matches. The header line stays visible. The lines from `return` to the closing brace are hidden.

```go
if err := load(); err != nil {
	return err
}
```

Does not match. `fold-body` needs `return` on a line after the opening brace.

```go
if err := load(); err != nil { return err }
```

With these settings:

```yaml
allow_bare_return: true
```

Matches.

```go
if err != nil {
	return
}
```

With these settings:

```yaml
allow_calls_in_results: true
```

Matches.

```go
if err != nil {
	return time.Now(), err
}
```

## noop

Methods that take no parameters, return nothing, and have an empty body. Hides: function declarations.

### Criteria

With the default settings, code matches when all of these hold:

- The function is a method. The receiver may be unnamed. Value, pointer, and generic receiver types all count.
- It takes no parameters.
- It has no results.
- It has a body, and the body holds no statements (`{}`). Comments in the body are allowed and hidden with it. A declaration without a body, such as a method implemented in assembly, does not match.

### Settings

| Setting | Type | Default | Description |
|---|---|---|---|
| `paths` | []string | none | Globs that limit the files this preset applies to, on top of `files.include` and `files.exclude`. |
| `exclude_paths` | []string | none | Globs of files this preset skips, on top of `files.exclude`. |
| `include_doc` | bool | true | Hide the doc comment together with the function. |
| `allow_comments` | bool | true | Hide the method even when a comment other than the doc comment is on its lines. |
| `include_functions` | bool | false | Also hide empty functions that are not methods, such as `func noop() {}`. |

### Examples

With the default settings:

Matches. A marker method from `go/ast`.

```go
func (*BadExpr) exprNode() {}
```

Matches.

```go
func (s Set[T]) sealed() {}
```

Matches.

```go
func (t *noopTracer) Flush() {}
```

Matches. A comment in the body does not matter by default.

```go
func (s *Server) Shutdown() { /* TODO: implement */ }
```

Does not match. The body has a statement.

```go
func (t *Tracer) Flush() { t.buf.Reset() }
```

Does not match. It takes parameters.

```go
func (nopLogger) Printf(format string, args ...any) {}
```

Does not match. It returns a value.

```go
func (nopCloser) Close() error { return nil }
```

Does not match. It is not a method.

```go
func noop() {}
```

Does not match. It has no body (implemented in assembly, for example).

```go
func (t *Timer) stop()
```

With these settings:

```yaml
allow_comments: false
```

Does not match. `allow_comments: false` keeps methods with comments visible.

```go
func (s *Server) Shutdown() { /* TODO: implement */ }
```

With these settings:

```yaml
include_functions: true
```

Matches. `include_functions: true` adds functions that are not methods.

```go
func noop() {}
```
