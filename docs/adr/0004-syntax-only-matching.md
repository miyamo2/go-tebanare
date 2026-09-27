# 0004 Match on the syntax of the changed file only

Status: Accepted

## Context

Code could be matched with full type information (`go/types`), which needs
the other files of the package and its imports. In the extension each extra
file is another request to GitHub.

## Decision

The analyzer parses only the old and new versions of the changed file and
matches on syntax.

## Consequences

- Results do not depend on files outside the diff, and every adapter gets
  the same result from the same two sources.
- The getter preset treats `return u.x` as a field access unless the same
  file declares a method `x` on the same type. A survey of the Go 1.24.7
  standard library with `go/types` found 337 such methods and no method
  value among them.
