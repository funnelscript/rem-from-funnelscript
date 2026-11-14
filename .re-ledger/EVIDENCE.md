# Evidence register

| ID | Classification | Baseline and conditions | Source or artifact reference | Claim or observation | Limits and next check |
| --- | --- | --- | --- | --- | --- |
| `E-001` | `observed` | Unit test in `internal/fetch` | `http_test.go:TestStealthHeaders` | Stealth client injects Chrome macOS headers, Sec-CH-UA, and Google Referer | Passes with `-race` |
| `E-002` | `observed` | Unit tests in `internal/providers` | `free_test.go:TestRedditPublic...`, `TestGitHubPublic...` | Zero-cost collectors parse Reddit JSON search and GitHub repo metrics without API keys | Passes with `-race` |
| `E-003` | `observed` | Unit test in `internal/collect` | `collect_test.go:TestWebsiteCollectsCareers...` | HTML collector extracts growth roles and enterprise pricing signals from `/careers` | Passes with `-race` |
| `E-004` | `observed` | Unit test in `internal/report` | `report_test.go:TestReportGeneratesScorecard...` | Scorecard rules rate open-source and reddit channels with cited evidence IDs | Passes with `-race` |
| `E-005` | `observed` | Unit test in `internal/app` | `service_test.go:TestDoctorReportsAllComponents` | Service doctor reports status of free providers, stealth mode, and keyless options | Passes with `-race` |
| `E-006` | `observed` | CLI flags and live binary execution | `main_test.go:TestNewFlagsParsed`, `./rem linear.app --keyless --stealth` | CLI binary executes end-to-end against live target and exits 0 with 6 verified evidence items | Live network dependent |
| `E-007` | `observed` | Web server test in `internal/web` | `server_test.go:TestStartRunAcceptsStealthKeylessAndWindow` | HTTP API parses stealth, keyless, and window JSON payloads without rejection | Passes with `-race` |
| `E-008` | `observed` | Unit test in `internal/collect` | `creative_test.go:TestDeconstructCreativeClassifiesAngles` | REA classifier categorizes agitation, us-vs-them, and founder angles with format normalization | Passes with `-race` |
| `E-009` | `observed` | Unit test in `internal/report` | `entrepreneur_test.go:TestAnalyzeEntrepreneur...` | Bayesian grievance engine scores time decay and outputs grounded Day-1 displacement plays | Passes with `-race` |
| `E-010` | `observed` | Unit test in `internal/app` | `service_test.go:TestInvestigationPopulates...` | Service pipeline populates `b.Entrepreneur` and doctor reports `creative_angles` and `entrepreneur_grievances` | Passes with `-race` |
