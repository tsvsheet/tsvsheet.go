# template.cli

The canonical urfave/cli v3 Go project template — focused on the **layout standard** (strict three-tier `app → domain → implementation`, file-per-tier, naming/import conventions, the `rename` scaffolding). The generic CLI framework it once embedded now lives in shared gomatic libraries it consumes:

- [`gomatic/go-app`](https://github.com/gomatic/go-app) — the urfave/cli framework (`Default`, `GetLogger`, global flags, `Run`).
- [`gomatic/go-log`](https://github.com/gomatic/go-log) — slog setup (`LoggerConfig`, `NewLogger`).
- [`gomatic/go-output`](https://github.com/gomatic/go-output) — JSON/YAML result encoding (via go-app).
- [`gomatic/go-error`](https://github.com/gomatic/go-error) — the sentinel `error.Const` type; this repo declares only its own values in `internal/constants`.

The template demonstrates the layout + how to consume these libraries; it does not re-implement or re-test them.
