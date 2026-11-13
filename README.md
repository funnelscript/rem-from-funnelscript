# REM From FunnelScript

REM researches how a company acquires customers from a public website, App Store listing, or Google Play listing. Each run saves a Markdown report and a JSON evidence bundle. The CLI, local web app, and MCP server use the same collectors and run history.

[![CI](https://github.com/funnelscript/rem-from-funnelscript/actions/workflows/ci.yml/badge.svg)](https://github.com/funnelscript/rem-from-funnelscript/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/funnelscript/rem-from-funnelscript)](https://github.com/funnelscript/rem-from-funnelscript/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Install

Download the archive for your platform from [GitHub Releases](https://github.com/funnelscript/rem-from-funnelscript/releases/latest). Compare its SHA-256 hash with `SHA256SUMS`, extract it, and put `rem` or `rem.exe` on your path. The macOS archives contain unsigned binaries.

With Go installed, you can build and install directly:

```sh
go install github.com/funnelscript/rem-from-funnelscript/cmd/rem@v0.1.0
rem demo
```

## Run it

From the source directory with Go 1.26 or later:

```sh
go build -trimpath -o rem ./cmd/rem
./rem doctor
./rem linear.app --only website,seo --no-ai --out ./rem-reports/linear
./rem validate ./rem-reports/linear/evidence.json ./rem-reports/linear/report.md
./rem web
```

`./rem demo` opens a recorded Cal AI investigation with no requests to source providers. Use `--no-open --port 0` to print a dynamically allocated loopback URL without launching a browser. Choose **Evidence only** in the browser to collect and view reports without a model.

To compare two snapshots:

```sh
./rem linear.app --only website,seo --no-ai --json
./rem linear.app --only website,seo --no-ai --json
./rem diff linear.app
```

`rem diff` also accepts two run IDs, two evidence files, or two report directories. An item absent from a later checked sample is described as *no longer seen in that sample*. It is not proof that an advertiser stopped running it.

## Sources and coverage

| Area | Evidence collected |
| --- | --- |
| Website and funnel | Homepage messaging, CTAs, pricing and trial signals, linked social accounts and apps, affiliate and team pages, logo candidates |
| Tracking | Public page and bounded Google Tag Manager tag signatures, attribution and marketing stack |
| SEO and AI search | Robots rules, `llms.txt`, nested and gzip sitemaps, URL families, locales, and update dates |
| History | Wayback snapshot availability and sampled changes in homepage messaging |
| App stores and launches | App Store metadata and ratings, Google Play public install bands, Apple EU Ad Repository, Hacker News |
| Founders | Public website hints and matched Wikidata claims. Optional paid provider enrichment for LinkedIn and X |
| Paid and social | With Scrape Creators, Meta, Google and LinkedIn ad samples, TikTok, Instagram, YouTube, X and LinkedIn, Reddit and Product Hunt |

Collectors label unavailable data as unknown and include manual-check links where possible. Tracking tags signal a possible channel. Ad samples and public install bands do not establish conversions, ROAS, spend, or causal impact.

`--only` accepts `website,seo,history,apps,paid,social,founders,community`. The website stage always runs when a public website is available. For store input the store lookup also runs first. Use `--handle tiktok=example` to override a discovered handle. If no observations are collected, the CLI exits nonzero and retains the run's source statuses and unknowns.

### Optional providers

- `SCRAPECREATORS_API_KEY` (alias `SCRAPE_CREATORS_API_KEY`) enables paid-source collection. `--max-credits` caps dispatched requests per run (default 50, range 0 to 500). The JSON credit count reports request reservations, which includes failed dispatched requests. Provider billing can differ. Cached successful responses may be up to 24 hours old and do not use a request reservation. Set `REM_NO_CACHE=1` before a comparison that requires fresh paid-source observations.
- `ANTHROPIC_API_KEY` or `ANTHROPIC_AUTH_TOKEN` enables the Anthropic writer. `--engine claude-code` and `--engine codex` use the installed and authenticated CLI. `--model` can override the selected writer's model. `--no-ai` and CLI `--json` use the deterministic evidence writer.
- Strategy writers receive the evidence needed to draft the report. A failed request or validation falls back to a saved evidence report and records a writer warning. Reports and provider data are local unless you choose an external writer. Configure credentials through your shell environment.

History defaults to the operating system's user config directory under `reverse-engineer-marketing-go/runs`. Set `REM_HOME` or pass `--home` to use another location. The local web app listens on IPv4 loopback. It exposes history, run progress and cancellation, report and evidence downloads, and content review. Jobs are local to the running process. Completed runs persist across restarts.

## Use with an MCP client

```sh
./rem setup --dry-run
codex mcp add reverse-engineer-marketing-go -- /absolute/path/to/rem mcp
```

Use the binary's stable absolute path. The same stdio command works with other clients:

```json
{"mcpServers":{"rem-go":{"command":"/absolute/path/to/rem","args":["mcp"]}}}
```

MCP offers investigation, evidence retrieval, content filtering, playbooks, report validation, report writing, and run comparison. Use `get_evidence` with `include_unknowns: true` to retrieve unresolved checks. The named prompts cover company investigation, comparison, monitoring, planning, and unknowns.

## Contribute

See [CONTRIBUTING.md](CONTRIBUTING.md) for development and release checks, [CHANGELOG.md](CHANGELOG.md) for changes, and [SECURITY.md](SECURITY.md) for private security reporting.

## Verify

```sh
go mod verify
go vet ./...
go test -race -count=1 -cover ./...
go build -trimpath -o rem ./cmd/rem
bash smoke.sh ./rem
```

For live verification use `REM_LIVE_TARGET=linear.app bash smoke.sh ./rem`. Direct outbound DNS and HTTPS are required by the public fetcher. Cross-compilation examples and platform checks are recorded in [the verification ledger](.re-ledger/VERIFICATION.md).

This is a new Go implementation with an original browser interface. Compatibility is measured by the workflow and evidence contracts in [the parity matrix](.re-ledger/PARITY.md). API field shape and collection limits can differ from the TypeScript release. Paid API and authenticated strategy adapters have fixtures and require real credentials for live confirmation.

The upstream project, embedded analyst playbooks, and recorded Cal AI demo are by Daniel Hangan (DansUGC), under MIT. See [LICENSE](LICENSE) and [the pinned reference](https://github.com/dansugc/reverse-engineer-marketing/tree/5b78198b2578997412853ab0858b10670fa74984).
