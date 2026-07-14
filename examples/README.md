# Example worksheets

Each example is a two-file worksheet — a `.tsv` value grid and its `.tsvt` template — ready to open in the browser spreadsheet. Serve one and edit it live (data-cell edits recompute the formulas through the same engine):

```sh
tsvsheet serve --template examples/grades.tsvt --data examples/grades.tsv
# then open http://127.0.0.1:8080
```

Every example also renders straight to stdout — handy for a terminal demo or piping into other tools:

```sh
tsvsheet render --template examples/invoice.tsvt --data examples/invoice.tsv | column -t
```

| Worksheet | Demonstrates |
| --- | --- |
| [grades](grades.tsvt) | Header-named columns, `avg`/`round`, and `if(...)` returning string results (`Pass`/`Fail`). The `Result` column reads the `Average` column computed earlier in the same row. |
| [invoice](invoice.tsvt) | Per-row arithmetic (`Amount = Qty * Price`) and a `=final` section that appends a `Total` row and sums the amount column with an absolute range. |
| [weather](weather.tsvt) | A same-row range (`High - Low`) and a row-relative reference (`High - High₋₁`, "vs. yesterday"). The first day has no prior row, so its cell is `#REF!` — the intended out-of-grid result, not a bug. A `=final` section appends daily averages. |
| [math](math.tsvt) | Error-value propagation: dividing by a zero denominator yields `#DIV/0!`, which flows through any expression that reads it. |
| [squares](squares.tsvt) | The minimal form — no section markers, so every line is a body line applied to each row (`n²` and `2n`). |

## A note on the language

The data lives in the `.tsv`; the `.tsvt` template carries only headers, formulas, and sheet operations. A few rules worth knowing when you edit these:

- A bareword header label is letters only (`Average`), so a name like `Test1` must be quoted (`"Test1"`) — otherwise `1` is read as a separate token.
- `<column> + <number>` is a *row* reference (`B+1` is "column B, one row down"), never arithmetic. To add a literal, put it first (`10 + B`) or parenthesize the reference (`(B) + 10`).

The full language is specified in [uplang/tsvsheet](https://github.com/uplang/tsvsheet); the choices this implementation makes for anything the specification left open are recorded in [specs/decisions/0003-open-semantics.md](../specs/decisions/0003-open-semantics.md).
