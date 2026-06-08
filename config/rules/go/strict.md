# Rule: go/strict

Go stack conventions (contributed by `stack: go`).

- `gofmt`/`goimports` clean; `go vet` and `golangci-lint` pass before commit.
- Hermetic unit tests; docker-backed tests behind the `integration` build tag.
- Errors checked; public APIs documented at the package or group level.
