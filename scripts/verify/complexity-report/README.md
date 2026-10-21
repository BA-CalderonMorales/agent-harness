# Complexity report

A read-only snapshot of the repository's shape: source size, function
length, cyclomatic complexity, and per-package aggregates. It exists so
"this package is getting hard to change" is a measurement rather than an
impression, and so an agent can find the hot spots without reading 46k
lines first.

Run it through the repository Makefile:

```bash
make complexity
```

Or directly:

```bash
cd scripts/verify/complexity-report
go run . -root ../../..
```

Useful flags:

| Flag | Default | Meaning |
|---|---|---|
| `-root` | `.` | Repository root to walk |
| `-top` | `20` | Rows in the ranked lists |
| `-budget` | `400` | Soft line budget per source file (house rule) |
| `-max-complexity` | `0` | With a non-zero value, fail when a function exceeds it |
| `-strict` | off | Also fail on files over `-budget` |
| `-json` | off | Machine-readable output |

Definitions:

- **Cyclomatic complexity** is `1 +` every `if`, `for`, `range`, `case`,
  `select` clause, and every `&&`/`||`. It approximates the number of
  independent paths a change has to reason about.
- **Tests are excluded** from source metrics and reported separately. The
  tool answers how hard the shipping code is to change, not how much test
  code exists.

The report is advisory by default. Raising `-max-complexity` in `make
complexity` is how the project can ratchet the worst offenders down over
time without freezing every refactor behind a red gate.
