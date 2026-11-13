# Verification

Implementation revision `65ef17b1beff2ebed39d3a5275dfe41dcd7530dd`. The handoff commit records these results and changes ledger files only. Upstream revision `5b78198b2578997412853ab0858b10670fa74984`.

## Native results

| Check | Conditions | Observed result |
| --- | --- | --- |
| Upstream baseline | Apple Silicon, Bun 1.4.3, frozen install, typecheck, test, eval, build | Exit 0. 23 tests and recorded Cal AI eval score 1.00 |
| Go checks | Linux x86-64, Go 1.26.5, `go mod verify`, `go vet ./...`, `gofmt`, `go test -race -count=1 -coverprofile=/tmp/rem-go.cover ./...` | Exit 0. 69 test cases including subtests, 70.1% statement coverage, 81.3% providers, 72.9% MCP |
| Go checks and CLI | Apple Silicon, Go 1.26.5, `go mod verify`, `go vet ./...`, `go test -race -count=1 ./...`, `REM_LIVE_TARGET=linear.app bash smoke.sh ./rem` | Exit 0. Two native Linear investigations produced reports, validator scored 1.0 and diff agreed on 1,020 sitemap URLs across both runs |
| Matched Linear baseline | Same public domain and website/SEO scope on Mac, upstream Bun and delivered Go executable | Both had 9 observations and 0 unknowns. Both found 1,020 sitemap URLs, 3 page families and llms.txt. Evidence ordering and prose vary |
| Notion end-to-end | Native Mac CLI, default areas, `notion.so --no-ai` | Resolved notion.com, 15 evidence items, 14 unknowns, report validation passed. Two Wikidata founders, 71 indexed Wayback months with five captures read, 29 matching HN submissions in 30 results, 18,750 sitemap URLs within caps, and an empty Apple EU ad check for the identified app |
| Store inputs | Native Mac CLI, Yelp App Store and Google Play URLs, `--only website --json` | Both returned real store evidence and resolved yelp.com. Apple reported 2,109,928 ratings. Google Play reported 50M+ downloads. Yelp's homepage returned 403, captured as unknown in both runs |
| MCP stdio interoperability | Linux binary, Python MCP SDK 1.29.0 | Protocol 2025-11-25, 7 tools, 5 prompts, two persisted runs, evidence retrieval, two comparison routes, recorded Cal AI with 23 evidence items and four unknowns, validation true. Direct Linux DNS returned network unreachable and was surfaced in source unknowns |
| MCP live | Native Mac binary, JSON-RPC stdio handshake, tool catalog, Linear website/SEO investigation | Protocol 2025-11-25, 7 tools, 9 live evidence items and 0 unknowns, saved run ID |
| Browser | Native Mac binary built from the implementation revision, headless Chromium, 390 × 844 | Passed live Linear website run with 7 evidence items, saved history, demo, clickable citations in report and scorecard, evidence search, downloads, two-run compare and single-run empty state. Fixture verified content filters, sorting and MP4 element. Zero page errors |
| Build targets | Go 1.26.5, CGO disabled | macOS arm64 and amd64, Linux arm64 and amd64, Windows amd64 all built. Mac arm64 native workflows and Linux amd64 CLI/MCP executed. Other binaries cross-compiled only |
| Paid provider | Mock HTTP transport against documented endpoint shapes | 13 focused fixture tests pass under the race detector. Covers request budget, keyed cache, response shape, and provider redirect handling. No paid requests in the acceptance runs |
| Authenticated report writers | Anthropic request fixture, local CLI argument review, deterministic report and validator checks | Credentialed provider execution remains pending |

Evidence comes from real runs unless stated as a fixture or compile check. Public counts can change after verification. The source-shaped snapshots intentionally have bounded samples.

## Reproduce

```sh
go mod verify
go vet ./...
go test -race -count=1 -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go build -trimpath -o rem ./cmd/rem
REM_LIVE_TARGET=linear.app bash smoke.sh ./rem
./rem notion.so --no-ai --out ./rem-reports/notion
./rem validate ./rem-reports/notion/evidence.json ./rem-reports/notion/report.md --json
./rem demo --no-open --port 0
```

For a cross-build, use `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o rem ./cmd/rem` and substitute the target OS and architecture. Use `.exe` for Windows.

Use only a dedicated configured account and a bounded request cap to close the paid-source check. Compare evidence to the original provider's UI or API for the selected brand. Separately generate one authenticated strategy report and verify the citation validator and fallback path.
