# Contributing

Build with Go 1.26 or later and use the latest patch release in that series for local development.

```sh
gofmt -w cmd internal
go mod verify
go vet ./...
go test -race ./...
go build -trimpath -o rem ./cmd/rem
bash smoke.sh ./rem
```

Start behavior changes with a test that fails against the old implementation. For external providers, use bounded, representative fixtures. Include the observed response contract and any uncertainty in the pull request. Keep tokens, account identifiers, local paths, and private reports out of fixtures and commits.

The CLI, browser and MCP server share `internal/app`. Collectors in `internal/collect` and `internal/providers` return observations, derivations and unknowns. Put interpretation and presentation in `internal/report`.

Maintainers publish a release by tagging a reviewed commit already reachable from `main`. The Release workflow tests that commit, builds the archives and uploads the checksums. Update the CLI version and release notes before tagging. Public releases use the MIT license.
