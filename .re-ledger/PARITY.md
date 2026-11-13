# Functional parity matrix

Passing behavior is scoped to the stated test. Live source records may change over time.

| ID | Priority | Scenario | Evidence | Verification | Status |
| --- | --- | --- | --- | --- | --- |
| P01 | Core | Public URL → website facts → evidence bundle | R02, R11 | Direct Mac Linear run | pass |
| P02 | Core | App Store and Google Play input resolves app metadata and developer site when available | R02, R06, R12 | Native Yelp store runs and provider fixtures | pass |
| P03 | Core | Keyless website, pricing, tracking, robots, nested sitemap and GEO | R05, R06, R11 | Collector fixtures and Mac Linear | pass |
| P04 | Core | Blocked, failed and empty sources remain distinguishable | R05 | HTTP/provider fixtures and observed Linux DNS failure | pass |
| P05 | Core | Reports resolve evidence IDs and reject malformed strategy claims | R05 | Validator tests, CLI report validation | pass |
| P06 | Core | Separate runs persist across restarts and compare without inferring an item stopped | R08 | Concurrent save, diff tests and repeated CLI runs | pass |
| P07 | Core | MCP initialize, catalogs, investigation, retrieval, validation, diff | R02, R13 | Native Mac MCP investigation, Go tests, Python MCP SDK | pass |
| P08 | Core | Browser run, evidence drill-down, history and downloads | R02, R11 | HTTP tests and Mac Chromium live run and comparison | pass |
| P09 | Core | Captured demo opens without network collection or keys | R02, R13 | Browser Cal AI fixture | pass |
| P10 | Secondary | Apple EU Ad Repository, Wayback, HN and Wikidata | R06, R12 | Provider fixtures and native Notion run | pass |
| P11 | Secondary | Scrape Creators keyed collectors and credit/cache behavior | R07 | Provider fixtures, including concurrency, redirects and cache | pass |
| P12 | Secondary | Live Scrape Creators account and quota behavior | R07 | Configured provider credentials and paid request needed | untested |
| P13 | Secondary | Authenticated Claude Code, Codex, Anthropic strategy output | R05 | Anthropic request fixture and local CLI argument checks. Authenticated report generation pending | untested |
| P14 | Secondary | Setup plan matches installed clients | R02 | CLI dry run. Writing to user configuration is separate | pass |
| P15 | Secondary | Exact upstream UI, file schema and per-provider search breadth | R05, R06 | Intentional new UI and evidence extensions. Broader query parity remains open | partial |
