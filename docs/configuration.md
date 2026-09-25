# Configuration

go-tebanare reads its configuration from `.gotebanare.yml` at the root of the repository, or from `.gotebanare.yaml` when the first file does not exist. When both exist, it uses `.gotebanare.yml` and reports a warning. The Chrome extension reads the file from the base branch of the pull request, so a pull request cannot change the configuration that applies to it.

Related pages:

- [presets.md](presets.md): the criteria, settings, and examples of each built-in preset.

## Example

```yaml
# .gotebanare.yml
version: 1

files:                      # when left out, include is ["**/*.go"]
  include: ["**/*.go"]
  exclude: ["vendor/**", "third_party/**"]

presets:                    # built-in rules, enabled by name
  - getter
  - noop
  - iferr:                  # "name: settings" changes the settings of a preset
      names: [err, "*Err"]
```

## Keys

### Top-level keys

| Key | Type | Required | Description |
|---|---|---|---|
| `version` | int | yes | Version of the configuration format. Must be `1`. |
| `files.include` | list of globs | no | Files to analyze. The default, also used for an empty list, is `["**/*.go"]`. |
| `files.exclude` | list of globs | no | Files to skip even when `include` matches them. |
| `presets` | list | no | Built-in presets to enable. See [presets](#presets). |

### Paths

- Globs follow the [doublestar](https://github.com/bmatcuk/doublestar) syntax, where `**` matches any number of directories. They match the path relative to the repository root, with `/` as the separator, and are case-sensitive.
- go-tebanare analyzes only files whose names end in `.go`, whatever `files.include` says.
- A file is analyzed when it matches `files.include` and does not match `files.exclude`. A preset with `paths` applies only to files that match one of them, and a preset with `exclude_paths` skips the files that match one of those.
- For a renamed file, go-tebanare checks the old version against the old path and the new version against the new path. [Pairing old and new versions](#pairing-old-and-new-versions) covers the case where the two paths get different answers.

## Presets

A preset is a built-in rule that you enable by name. This configuration enables all three:

```yaml
# .gotebanare.yml with every preset
version: 1
presets:
  - getter
  - noop
  - iferr
```

| Preset | Hides |
|---|---|
| `getter` | Methods that only return a field of the receiver, such as `func (u *User) Name() string { return u.name }`. |
| `noop` | Methods that take no parameters, return nothing, and have an empty body. |
| `iferr` | `if err != nil` blocks that return the error unchanged. |

To change the settings of a preset, write `name: settings` in place of the name:

```yaml
presets:
  - getter                    # default settings
  - noop:
      include_functions: true # also hide empty functions that are not methods
  - iferr:
      names: [err, "*Err"]    # also parseErr and similar names
      paths: ["internal/**"]  # limit this preset to some files
```

- `- iferr:` with no value and `- iferr: {}` are the same as `- iferr`. Settings that you leave out keep their defaults.
- Listing a preset twice is an error. An unknown preset name is an error whose message lists the available presets.
- An unknown setting is an error whose message lists the available settings. A value of the wrong type is an error, and so is an integer setting that does not fit in a signed 32-bit integer.
- The UI shows the preset name, such as `getter`, as the rule id.

Settings shared by the presets:

| Setting | Default | Meaning |
|---|---|---|
| `paths` / `exclude_paths` | none | Limit the files of this preset, on top of `files.include` and `files.exclude`. |
| `include_doc` | `true` | Only for presets that hide functions (`getter`, `noop`): hide the doc comment together with the function. |

[presets.md](presets.md) lists the criteria, settings, and examples of each preset. In the criteria, "comment" means any comment on the hidden lines other than the doc comment, trailing comments included.

## Validation

go-tebanare decodes the configuration strictly and reports every error it finds, each with its position. These are errors:

- A YAML syntax error, a document that is not a mapping, or a second YAML document with content. Empty documents, such as the one after a trailing `---`, are skipped.
- An unknown key at any level, such as `preset` for `presets` (the message lists the allowed keys), a key set twice, a merge key (`<<`), or a value of the wrong type. A `null` value counts as unset.
- A missing `version`, or a `version` other than `1`.
- An invalid glob in `files`, `paths`, or `exclude_paths`.
- In `presets`: an unknown preset, a preset listed twice, or invalid settings.
- Aliases that copy more than 64 KiB in total, or an alias inside the value it refers to.

Most errors have the form `file:line:column: field: message`. Errors about the whole file leave out the field, and a YAML syntax error gives only the line. With `getter` misspelled as `geter` in the [example](#example), the error reads:

```text
.gotebanare.yml:9:5: presets[0](geter): unknown preset "geter" (available presets: getter, iferr, noop)
```

The Chrome extension cannot show the parser's message for a YAML syntax error. yaml.v3 reports syntax errors by panicking, and the WebAssembly build of the engine stops instead of recovering. The extension reports a `config-syntax` error on the line where the parser stopped and hides nothing.

## What gets hidden

A diff shows whole lines, so go-tebanare hides whole lines. It processes each version of a file in these steps:

1. Skip the file when it is larger than 1 MiB (`too-large`).
2. Count bracket nesting and `else if` chains with the Go scanner, and skip the file without parsing it when brackets nest deeper than 200 levels or a chain has more than 1,000 links (`too-deep`).
3. Parse the file. On any parse error, hide nothing in it (`parse-error`).
4. Skip the file when its syntax tree is deeper than 1,500 levels (`too-deep`).
5. Apply the presets. Line numbers are lines of the file itself: `//line` directives are ignored.
6. Keep each match that passes the occupancy check below.
7. Merge the ranges. Ranges separated only by blank lines become one fold.

### Occupancy check

A match is hidden only when its lines hold no other code. On its first line, only whitespace, comments, `,`, and `;` may come before the matched code, and on its last line only those may come after it. Otherwise the match stays visible, and a `line-shared` diagnostic names the preset and the line.

```go
func (s *Store) Name() string { return s.name }; var defaultName = "main" // getter stays visible: the line holds a var declaration too

func (s *Store) Flush() {}; func (s *Store) Reset() {} // neither noop match is hidden: each one shares the line with the other

func (s *Store) Close() {} // hidden: the method occupies its line

func (s *Store) Save() error {
	err := s.write()
	if err != nil { return err }; s.saved++ // iferr stays visible: s.saved++ shares the line
	return nil
}
```

### Hidden lines per preset

| Preset | Hidden lines |
|---|---|
| `getter`, `noop` | From the first line of the doc comment (with `include_doc: true`, the default) to the last line of the declaration. When any line of the doc comment is a directive, the doc comment stays visible and hiding starts at the `func` line. |
| `iferr` | From the `if` line to the closing brace. With `init: fold-body`, an `if` statement with an init statement keeps its first line visible, and hiding starts at the `return` line. |

A directive is a line comment that starts with `//line `, `//export `, or `//extern `, or that has the form `//word:x...`, where `word` and the first character after the colon are lowercase ASCII letters or digits (`//go:linkname`, `//nolint:errcheck`). `//todo: fix` is not a directive because a space follows the colon. go/ast uses the same rule.

## Pairing old and new versions

go-tebanare analyzes the old and new versions of a file separately and pairs the matches of the presets that hide functions, `getter` and `noop`.

- Declarations are paired by key: `Recv.Name` for methods, using the base type name of the receiver, and `Name` for functions. `init` functions and declarations named `_` are paired in order of appearance.
- When one version declares the same key twice (valid syntax that does not compile), none of those declarations is hidden in either version, and a `duplicate-decl` diagnostic reports it.

| Old version | New version | Result |
|---|---|---|
| Matches | Matches | Both are hidden, including moves and body changes. |
| Matches | Not declared | The old one is hidden (a deletion). |
| Not declared | Matches | The new one is hidden (an addition). |
| Matches | Declared, does not match (or the reverse) | Neither is hidden, and a `match-changed` diagnostic reports it. A change of match state needs review, for example a getter that gained logic. |

- A match that fails the occupancy check still counts as a match for pairing, but neither it nor its counterpart is hidden.
- When the `paths` or `exclude_paths` of a preset select only one of the two paths, the declaration in the other version counts as declared without a match for that preset. That preset then hides the function in neither version.
- `iferr` matches have no reliable pairing, so each version is matched on its own, and one version can hide lines that the other shows.
- A context line of the diff is hidden only when its line is hidden in both versions.
- When either version is skipped (`not-target`, `too-large`, `too-deep`, `parse-error`, or a stopped engine), nothing is hidden in either version. A rename where only one of the paths matches `files` counts as a skip.

## Safety behavior

When go-tebanare cannot be sure, it hides nothing.

| Situation | Behavior |
|---|---|
| The configuration does not parse or is invalid | Nothing is hidden, and the UI shows the errors with their position. For a YAML syntax error, the extension shows only the line where the parser stopped (see [validation](#validation)). |
| A Go file does not parse | Nothing is hidden in the file (`parse-error`). |
| A file is larger than 1 MiB | Nothing is hidden in the file (`too-large`). |
| Nesting is too deep: brackets past 200 levels, an `else if` chain past 1,000 links, or a syntax tree past 1,500 levels | Nothing is hidden in the file (`too-deep`). |
| The engine panics | In the browser, the WebAssembly engine stops; the extension starts a new one and hides nothing in that file. |
| A match shares its lines with other code, or its match state changed | That code stays visible, with a diagnostic. |

The size and depth limits are defaults of the engine and can change between releases.

## Compatibility

A configuration lives in your repository, so a change in how go-tebanare reads it would change what gets hidden without anyone editing the file. Version 1 fixes the following:

| Area | Fixed in version 1 |
|---|---|
| Configuration | Keys, value shapes, defaults, lookup order, and the conditions that are errors. |
| Presets | Criteria, settings, and default values. |
| Hidden lines | Occupancy check, doc comments, merging, and pairing of old and new versions. |

Diagnostic and warning messages (new warnings included), the size and depth limits, and the UI are outside this promise.

- No release of version 1 widens what gets hidden. Wider behavior arrives as a new key, setting, or preset that is off by default, or as `version: 2`.
- A preset never changes to hide more, and neither does the default of a setting. New behavior gets a new preset name, and a new setting keeps the old behavior at its default.
- A change that narrows what gets hidden happens only to fix a bug (behavior that contradicts this documentation) or a safety problem (code hidden that should stay visible), and the release notes list it.
- A new key, setting, or preset never changes the result of a configuration that does not use it.
- The preset examples are golden tests. When a Go or TinyGo update changes a golden result in a way that would hide more, the project either skips that update or, before taking it, adjusts the engine so that the result stays the same.
