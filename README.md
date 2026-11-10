# Veylqo — Less URL. More destination.

Self-hosted, open-source link shortener and share service. Go 1.27,
standard library first. The JS SDK lives in
[`jovrion/veylqo-js`](https://github.com/jovrion/veylqo-js).

This repository is early. Product code lands through the board in
`work/`; a task describes intent, not a feature that already exists.

## Clone

```
git clone https://github.com/jovrion/veylqo.git
cd veylqo
```

The Go module is `github.com/jovrion/veylqo` (`go 1.27`). There is no
service binary yet. Board commands need only Python 3 from the standard
library.

```
make fmt     # gofmt -s -w .
make vet     # go vet ./... (no-op until a package exists)
make test    # go test ./... (no-op until a package exists)
make build   # go build ./... (no-op until a package exists)
```

## How work is tracked

```
make work-next              # next ready task (phase, then priority)
make work-start ID=VYQ-10   # claim: open → in-progress (WIP = 1)
make work-done  ID=VYQ-10   # close when every Verification box is ticked
make work-validate          # schema, cycles, WIP, fresh BOARD.md
make work-view              # Kanban + map (NOOPEN=1 to only generate)
```

Rules: `WORKFLOW.md`, `work/README.md`, `AGENTS.md`.
OpenCode: `/work-next`, `/work-start`, `/work-done`.

## License

MIT. See `LICENSE`.
