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
| [squares](squares.tsvt) | The power operator (`=A2^2`, `=A2^3`) building square and cube columns, over a `Total` row that sums each column across a range (`=sum(B2:B6)`). |
| [weather](weather.tsvt) | Per-row differences (`Range = High − Low`, `=B2-C2`) and a `Peak` summary row reducing a column with `max`/`min` (`=max(B2:B6)`, `=min(C2:C6)`). |
| [functions](functions.tsvt) | A reference sheet demonstrating every built-in function with a worked formula and computed result — math and trig, aggregate and statistics, text, logical and info, date and time, lookup, financial, and a dynamic-array showcase whose results spill down their columns. |
| [order](order.tsvt) → [discount](discount.tsvt) | **Embedded sheets** — each `Line total` embeds the whole [discount](discount.tsvt) sheet as a function: `=sheet("discount.tsvt", C2, B2)` passes the unit price and quantity, and the cell's value is that sub-sheet's `=output(…)`. |

## Embedded sheets — a spreadsheet as a function

A cell can embed **an entire other sheet** and take its computed output as the cell's value, so a `.tsvt` becomes a reusable, parameterised function. Three builtins express it:

- **`output(expr)`** marks a cell as the sheet's single output (its value is `expr`).
- **`sheet(path, arg…)`** loads that sheet, computes it, and yields its `output` value; the extra arguments are passed in.
- **`input(n)`** reads the nth argument inside the embedded sheet.

So [discount.tsvt](discount.tsvt) reads `input(1)`/`input(2)`, computes a discounted total, and exposes it via `=output(C3)`; [order.tsvt](order.tsvt) embeds it per row. In the browser editor (`tsvsheet serve order.tsvt`), selecting an embedding cell shows the nested sub-sheet inline. Referenced paths resolve within the sheet's own directory; a cross-sheet cycle is `#CIRC!`, an unresolved path `#REF!`. (Run `discount.tsvt` on its own and its `input(…)` cells are `#REF!` — it is meant to be embedded.)

## A note on the language

A `.tsvt` **is** the spreadsheet: there is no separate data file. Each cell is a literal value, or — when it begins with `=` — a formula over the Excel-faithful expression sublanguage: arithmetic (`+ - * /`), power (`^`), text concatenation (`&`), postfix percent (`%`), comparisons (yielding `TRUE`/`FALSE`), number / string / boolean / error-value literals, and builtins like `sum`, `avg`, `min`, `max`, `count`, `round`, `abs`, `len`, `concat`, `mod`, `if`. Formulas reference other cells by A1 address, exactly like a conventional spreadsheet; a reference off the grid resolves to `#REF!`, a cycle to `#CIRC!`, division by zero to `#DIV/0!`.

Worth knowing when you edit these: references are A1 (`B2`, `$B$2`, ranges `D2:D5`); `%` is postfix percent (`50%` = 0.5), so modulo is the `mod(a, b)` function.

The full language is specified in [uplang/tsvsheet](https://github.com/uplang/tsvsheet).
