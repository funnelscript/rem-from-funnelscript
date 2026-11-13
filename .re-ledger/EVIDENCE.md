# Evidence and provenance

The upstream revision is `5b78198b2578997412853ab0858b10670fa74984`. Source paths below refer to that immutable [tree](https://github.com/dansugc/reverse-engineer-marketing/tree/5b78198b2578997412853ab0858b10670fa74984). Reproducible checks and platform distinctions are in VERIFICATION.md.

| ID | Status | Finding | Source or condition |
| --- | --- | --- | --- |
| R01 | observed | Upstream native baseline passed typecheck, 23 tests, recorded Cal AI evaluation (1.0) and build | Mac, Bun 1.4.3, `bun install --frozen-lockfile` and package scripts |
| R02 | observed | Shared website-first collection pipeline from CLI/MCP/web. Store inputs resolve metadata first | `src/cli.ts`, `src/index.ts`, `src/target.ts`, `src/mcp.ts`, `src/web/server.ts` |
| R03 | observed | REA 6.8.0 parsed four TypeScript surfaces without executing app bootstrap | Static evidence ID `ev_1eedb2dc39e8a2e2cad746db2a934bec1af06d8ba17d71c4da1ebcd3acacb4d6`, file SHA-256 `a3c55d604f18dc907c2c713c81643568d1db5c0dff547174bfc9da62481c99c9`, summary view complete for 320 selected elements |
| R04 | observed | Replica Square produced a bounded directory inventory with 172 entries. Static inventories complement runtime checks | `replica-square recon` on upstream checkout |
| R05 | documented | Observations and derivations cite sources, unsupported channels remain unknown, agents can fetch exact evidence and validate narrative | `AGENTS.md`, `src/evidence.ts`, `src/report/validate.ts`, `src/analyze/scorecard.ts`, `harness/manifest.json` |
| R06 | observed | Public website, tracking, robots/sitemaps, store lookups, Wayback, HN and provider interfaces identified in pinned collectors | `src/sources/` |
| R07 | documented | Paid coverage requires Scrape Creators. Relevant public endpoint contracts were checked against current docs for Meta, Google, TikTok, Instagram, LinkedIn and Reddit | `src/sources/scrapecreators.ts`, [provider docs](https://docs.scrapecreators.com/) |
| R08 | observed | Upstream stores snapshots and supports monitoring. Saved IDs and explicit unknowns must remain distinct | `src/runs.ts`, `src/report/diff.ts`, upstream tests |
| R09 | documented | Agent-Reach informed capability diagnostics, Scrapling informed bounded extraction and explicit fallback, last30days informed source/freshness handling | [Agent-Reach](https://github.com/Panniantong/Agent-Reach), [Scrapling](https://github.com/D4Vinci/Scrapling), [last30days](https://github.com/mvanhorn/last30days-skill) |
| R10 | documented | Go net/http transport supports pinned-address dialing, timeout and redirect control | Context7 `/golang/go`, installed Go 1.26.5 source |
| R11 | observed | Keyless Go run for Linear returned website and SEO evidence on Apple Silicon. Cloud Linux allowed build/fixtures but its direct DNS returned network unreachable | Initial integrated binary, Mac process 10827 |
| R12 | observed | Published API shape checks for iTunes, Play, HN, Apple EU Ads and Wikidata; native delivery smoke is tracked separately | Provider-specific HTTPS checks with the executor egress proxy and deterministic Go fixtures |
| R13 | observed | Official MCP client handshake, seven tools, five named prompts, two saved runs, and evidence/report retrieval worked over stdio | Python MCP SDK 1.29.0. [MCP spec](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports) |
