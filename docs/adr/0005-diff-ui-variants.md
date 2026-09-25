# 0005 Support GitHub diff views through isolated UI variants

Status: Accepted, pending spike S2

## Context

GitHub renders "Files changed" with a server-rendered table and with a
newer React view, and changes the markup without notice. A wrong mapping
from rows to line numbers could hide the wrong lines.

## Decision

Each markup gets its own `DiffUiVariant` that finds file containers, file
paths, and rows with their old and new line numbers. Everything after that,
such as planning which rows to hide and drawing folds, works on those rows
and knows no GitHub markup. Before hiding anything, the extension compares
each row's text with the line of the fetched source that has the same
number, and hides nothing in a file where one row differs.

## Consequences

- When no variant recognizes the page, the extension hides nothing and the
  popup reports an unsupported GitHub UI.
- The classic variant targets the server-rendered unified table. Its tests
  use synthetic fixtures modeled on that markup, because this environment
  cannot load github.com. Spike S2 must replace them with saved pages and
  check the selectors.
- The React variant reads the React view. Its selectors come from a saved
  page that shows modified files in unified view. Spike S2 still has to
  check deleted lines, added, deleted, and renamed files, review threads,
  and the split view on that markup.
