# 0005 Support GitHub diff views through isolated UI variants

Status: Accepted, pending spike S2

## Context

GitHub renders "Files changed" with a server-rendered table and with a
newer React view, and changes the markup without notice. A wrong mapping
from rows to line numbers could hide the wrong lines.

## Decision

Each markup gets its own `DiffUiVariant` that finds file containers, file
paths, file headers and statuses, whether a file shows the split view, and
rows with their old and new line numbers and text. Planning which rows to
hide works on those rows and knows no GitHub markup; the fold rows assume
only a table with two line number columns before the code.

Before hiding anything, the extension checks every row it is about to hide
against the analysis: the hash of the row's text must equal the hash of the
same line in the analyzed source (the old side for a deleted row, the new
side for an added row, both for a context row). If one row differs, it
hides nothing in that file and the banner says the page does not match the
analyzed source.

## Consequences

- When no variant recognizes the page, the extension hides nothing, and the
  banner and the popup report an unsupported GitHub UI.
- Neither variant reads the split view yet. A file shown split hides
  nothing, and the banner says the split view is not supported.
- The classic variant targets the server-rendered unified table. Its tests
  use synthetic fixtures modeled on that markup, because this environment
  cannot load github.com. Spike S2 must replace them with saved pages and
  check the selectors.
- The React variant reads the React view. Its selectors come from a saved
  page that shows modified files in unified view. Spike S2 still has to
  check deleted lines, added, deleted, and renamed files, review threads,
  and the split view on that markup.
