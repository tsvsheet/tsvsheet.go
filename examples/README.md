# Example sheets

Each example is a single `.tsvt` spreadsheet — a TAB-separated grid whose cells are literal values or `=formulas` that address other cells in A1 notation (`B2`, `D2:D5`). Open one in the browser editor and edit any cell live (edits recompute through the same engine):

```sh
tsvsheet serve examples/grades.tsvt
# then open http://127.0.0.1:8080
```

Every example also renders straight to stdout — handy for a terminal demo or piping into other tools:

```sh
tsvsheet render examples/invoice.tsvt | column -t
```

| Sheet | Demonstrates |
| --- | --- |
| [grades](grades.tsvt) | Per-row aggregates (`round(avg(B2:D2), 1)`) and a conditional text result (`if(E2 >= 70, "Pass", "Fail")`) that reads the average computed earlier in the same row. |
| [invoice](invoice.tsvt) | Per-row arithmetic (`Amount = Qty × Price`, `=B2*C2`) and a `Total` row summing the amount column over a range (`=sum(D2:D5)`). |
| [math](math.tsvt) | Error-value propagation: dividing by a zero denominator yields `#DIV/0!`, which flows through any expression that reads the cell. |

## A note on the language

A `.tsvt` **is** the spreadsheet: there is no separate data file. Each cell is a literal value, or — when it begins with `=` — a formula over the Excel-faithful expression sublanguage: arithmetic (`+ - * /`), power (`^`), text concatenation (`&`), postfix percent (`%`), comparisons (yielding `TRUE`/`FALSE`), number / string / boolean / error-value literals, and builtins like `sum`, `avg`, `min`, `max`, `count`, `round`, `abs`, `len`, `concat`, `mod`, `if`. Formulas reference other cells by A1 address, exactly like a conventional spreadsheet; a reference off the grid resolves to `#REF!`, a cycle to `#CIRC!`, division by zero to `#DIV/0!`.

Worth knowing when you edit these: references are A1 (`B2`, `$B$2`, ranges `D2:D5`); `%` is postfix percent (`50%` = 0.5), so modulo is the `mod(a, b)` function.

The full language is specified in [uplang/tsvsheet](https://github.com/uplang/tsvsheet).
