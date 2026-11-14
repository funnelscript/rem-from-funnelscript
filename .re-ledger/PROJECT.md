# REM Go — Project Brief

**Target source**: https://github.com/dansugc/reverse-engineer-marketing  
**Outcome**: New Real & Mature Go version — Upgrade the existing `reverse-engineer-marketing-go` to production-grade quality  
**Mode**: source-informed upgrade  
**Stack**: Go 1.26+, stdlib + golang.org/x/net, single binary, MCP, web UI, CLI  
**Done when**: Binary works end-to-end (live run, web UI, MCP), all tests pass, smoke.sh passes on a live target

## Existing State (verified 2026-11-14)
- `rem-from-funnelscript/` is the canonical working Go codebase (https://github.com/funnelscript/rem-from-funnelscript)
- `go build ./...` → EXIT 0
- `go test ./...` → all packages PASS (10 packages)
- `./rem doctor` → shows available engines, all free sources listed
- `./rem linear.app --only website,seo --no-ai --json` → 9 evidence items, works
- Binary `./rem` exists and is current

## Gaps Identified
1. **No install path**: no Makefile, no install target, no release binary workflow
2. **Version is 0.1.0 — no semver bumping in CI**
3. **No real-world end-to-end smoke test with live target** (smoke.sh has optional REM_LIVE_TARGET)
4. **Missing .re-ledger tracking** (this file is first entry)
5. **No README quickstart for contributors** beyond basic usage
6. **Upgrade opportunities from inspiration projects**:
   - Agent-Reach: multi-agent parallel collection dispatch pattern
   - Scrapling: browser-grade scraping fallback for JS-heavy pages
   - last30days-skill: structured playbook harness pattern
7. **Web UI lacks Tailwind/OKLCH polish** — functional but minimal

## Upgrade Plan (Phases)
- Phase A: Verify baseline, initialize ledger
- Phase B: Makefile + install + cross-compile + smoke-test automation
- Phase C: Collector hardening (GTM tag expansion, llms.txt enrichment, affiliate tracker detection)
- Phase D: Web UI improvement (better status, run history UX)
- Phase E: MCP tool expansion (validate_run, compare_runs tool)
- Phase F: Final verification + README update
