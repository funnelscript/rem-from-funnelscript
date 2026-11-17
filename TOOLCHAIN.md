# Tool Routing & Capability Map

Resolve each entry in the running host. A slash command is a host action. A skill is installed guidance. A CLI and an MCP tool have their own entry points and permissions. Record only relevant instructions, versions, and fallbacks in the ledger.

| Phase | Requested Tool | Job and Result | Fallback or Trigger |
|:---|:---|:---|:---|
| 0 | `/goal` | Set target, mode, acceptance criteria, and completion conditions | Put contract in brief and link from `STATUS.md` |
| 0 & Handoffs | Elephant-Goldfish Model (EGM) | Design with an elephant (deep context, draft, resolve) and review with a goldfish (`egm-zoom-out`, `egm-comprehension`, `egm-design`, `egm-implementation-readiness`) | Follow EGM guidelines, enforce artifact-first delivery |
| 0 to 6 | `/j-space` | Seam ledger and cognitive reasoning across phase boundaries | Record decisions, evidence, and next actions at every seam |
| 1 to 5 | `rem kb` (REM Knowledge Base CLI & MCP) | Scaffolding (`kb init`), validation (`kb check`), script delivery (`kb script`), report compilation (`kb report`), and coding agent bundling (`kb pack`) | Run `./rem kb` or invoke MCP tools `kb_init`, `kb_check`, `kb_build_report` |
| 1 & 4 | `tools/page/inventory.js` (`rem kb script inventory`) | Browser DOM extraction: frameworks, custom component tags, test-IDs, ARIA roles, normalized routes, and SDK globals | Evaluate via Playwright/CDP `Runtime.evaluate` |
| 1 & 4 | `tools/page/tokens.js` (`rem kb script tokens`) | Extracts CSS custom properties, computed styles, color frequencies, typography scale, radii, and shadows | Run via Playwright browser evaluate |
| 1 & 4 | `tools/page/mask.js` (`rem kb script mask`) | Two-tier physical PII redaction: DOM text/input replacement and dynamic CSS blur for safe clean-room screenshots | Inject before taking screenshots |
| 1 & 4 | `tools/page/annotate.js` (`rem kb script annotate`) | Visual bounding boxes and numbered badges on DOM landmarks for screen anatomy figures | Inject before capturing annotated screenshots |
| 1 | `/rea` & `/reverse-engineer-anything` | Forensic binary, Electron, Mach-O, ELF, APK, and runtime memory decompilation and network analysis | Use for native binaries or packaged desktop apps; use `rem kb` for SaaS web products |
| 1 | Scrapling Stealth Engine (`./rem --stealth`) | Realistic browser TLS/HTTP-2 fingerprinting, sec-ch-ua headers, Google referer spoofing, WAF/Cloudflare bypass | Use standard fetch client when target is unblocked |
| 1 | Agent-Reach Keyless Engine (`./rem --keyless`) | Zero-cost multi-platform search across Hacker News (Algolia API), Reddit public JSON, and GitHub public repos | Fall back to paid Scrape Creators API or manual search links |
| 1 & 2 | last30days Recency & Hiring Signals | 30-day temporal freshness gating, velocity calculation, and careers page scanning for growth/marketing hires | Use all-time evidence when historical analysis is requested |
| 1 to 4 | Context7 | Resolve and query relevant library docs for pinned dependency versions | Query official pinned docs or types |
| 2 & 5 | `/replica-square` | Orchestrate 10 tool-agents (`recon`, `har`, `entrepreneur`, `architect`, `design`, `diff`, `sweep`, `launch`, `mock`, `test-gen`) for clean-room reconstruction | Use `replica-square` CLI or MCP tools |
| 2 | `rem kb barriers` (`clone/barriers.md`) | Audit launch barriers, certifications (SOC 2, ISO 27001), gatekeepers, and moats | Run `./rem kb barriers <dir>` |
| 3 to 6 | Compound Engineering | 80/20 planning and review (`ce-brainstorm`, `ce-plan`, `ce-code-review`, `ce-compound`), codifying lessons into `docs/solutions/` | Apply `ce-plan` or `ce-compound` |
| 3 & 4 | get-fable (`/get-fable`) | Track mutation vs verified generation, enforce Fable Circuit Breaker, TDD red-green-refactor | Use project-native checks and REM task DAG |
| 4 & 5 | Poteto Mode (`/Poteto Mode`) | Domain data modeling, type system discipline, zero obviousness, and unslopped execution | Apply `/poteto-mode` playbooks |
| 3 to 5 | Anti-Slop Engineering (`/anti-slop-engineering`) | Gates 0–6, SRE principles (KISS, DRY, YAGNI, Fix-at-Scale), fail-closed security, no mock fallbacks on production paths | Audit with `bunx anti-slop-engineering` |
| 5 | `/openreview` | Autonomous PR review loop, triaging findings against tests and contracts | Run independent review before release |
| 6 | `BUILD.md` + Pack Handoff | Packaging knowledge base into a self-contained, sanitized zip handoff for downstream coding agents | Run `./rem kb pack <dir>` |
