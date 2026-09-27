# 0005 Support GitHub diff views through isolated UI variants

Status: Accepted

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
- The classic variant targets the server-rendered unified table. Spike S2
  replaced its synthetic fixtures with saved pages and confirmed the
  selectors.
- The React variant reads the React view. Spike S2 confirmed its selectors
  against saved pages covering deleted lines, added, deleted, and renamed
  files, review threads, and the split view.
