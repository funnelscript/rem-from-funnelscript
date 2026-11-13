package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/funnelscript/rem-from-funnelscript/internal/collect"
	"github.com/funnelscript/rem-from-funnelscript/internal/fetch"
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
	"github.com/funnelscript/rem-from-funnelscript/internal/providers"
	"github.com/funnelscript/rem-from-funnelscript/internal/report"
	"github.com/funnelscript/rem-from-funnelscript/internal/store"
)

var Areas = []string{"website", "seo", "history", "apps", "paid", "social", "founders", "community"}
var handlePattern = regexp.MustCompile(`^[A-Za-z0-9@_.-]{1,100}$`)

type Service struct {
	Store *store.Store
	get   fetch.Getter
	http  *http.Client
	now   func() time.Time
}

func New(s *store.Store) *Service {
	network := fetch.New()
	return &Service{Store: s, get: network, http: network.HTTP, now: time.Now}
}

func env(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func (s *Service) Doctor() map[string]any {
	_, claudeErr := exec.LookPath("claude")
	_, codexErr := exec.LookPath("codex")
	return map[string]any{
		"areas":          Areas,
		"version":        model.Version,
		"scrapeCreators": env("SCRAPECREATORS_API_KEY", "SCRAPE_CREATORS_API_KEY") != "",
		"engines":        map[string]bool{"evidence": true, "anthropic": env("ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN") != "", "claude-code": claudeErr == nil, "codex": codexErr == nil},
		"freeSources":    []string{"website", "tracking", "seo", "wayback", "appstore", "apple_ads", "hackernews", "founders"},
	}
}

func CheckOptions(opts Options) error {
	for _, a := range opts.Only {
		if !slices.Contains(Areas, a) {
			return fmt.Errorf("unknown area %q", a)
		}
	}
	if opts.MaxCredits < 0 || opts.MaxCredits > 500 {
		return errors.New("credit budget must be between 0 and 500")
	}
	for platform, handle := range opts.Handles {
		if !slices.Contains([]string{"tiktok", "instagram", "youtube", "x", "linkedin", "reddit", "threads", "producthunt", "facebook"}, platform) || !handlePattern.MatchString(handle) {
			return fmt.Errorf("invalid social handle for %q", platform)
		}
	}
	return checkEngine(opts.Engine, opts.Model)
}

type stageResult struct {
	source model.SourceRun
	result model.Result
}

func (s *Service) Investigate(ctx context.Context, input string, opts Options) (model.Bundle, error) {
	if err := ctx.Err(); err != nil {
		return model.Bundle{}, err
	}
	target, err := ParseTarget(input)
	if err != nil {
		return model.Bundle{}, err
	}
	if err = CheckOptions(opts); err != nil {
		return model.Bundle{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	now := s.now().UTC()
	b := model.Bundle{
		Tool: model.Metadata{Name: "reverse-engineer-marketing-go", Version: model.Version}, Harness: report.Harness(),
		GeneratedAt: now.Format("2006-01-02T15:04:05.000000000Z"), Evidence: []model.Evidence{}, Unknowns: []model.Unknown{},
		Sources: []model.SourceRun{}, Content: []model.Content{},
	}
	discovered := model.Discovered{Socials: map[string]string{}}
	maxCredits := opts.MaxCredits
	if !opts.BudgetSet && maxCredits == 0 {
		maxCredits = 50
	}
	paid := providers.NewPaid(s.http, env("SCRAPECREATORS_API_KEY", "SCRAPE_CREATORS_API_KEY"), maxCredits)
	paid.SetCacheDir(s.Store.CacheDir())
	var progressMu sync.Mutex
	progress := func(stage, state string) {
		if opts.OnStage == nil {
			return
		}
		progressMu.Lock()
		defer progressMu.Unlock()
		opts.OnStage(stage, state)
	}
	run := func(name string, call func() model.Result) stageResult {
		progress(name, "start")
		began := time.Now()
		result := call()
		sr := model.SourceRun{Source: name, Status: "ok", MS: time.Since(began).Milliseconds()}
		if len(result.Evidence) == 0 {
			sr.Status = "skipped"
			if len(result.Unknowns) > 0 {
				sr.Reason = result.Unknowns[0].Reason
				for _, u := range result.Unknowns {
					l := strings.ToLower(u.Reason)
					if strings.Contains(l, "http ") || strings.Contains(l, "failed") || strings.Contains(l, "unavailable") || strings.Contains(l, "invalid response") || strings.Contains(l, "timeout") {
						sr.Status = "error"
						break
					}
				}
			} else {
				sr.Reason = "No observations returned"
			}
		}
		state := "done"
		if sr.Status == "error" {
			state = "error"
		}
		progress(name, state)
		return stageResult{sr, result}
	}
	appendStage := func(stage stageResult) {
		b.Sources = append(b.Sources, stage.source)
		merge(&b, stage.result, now)
	}
	storeTarget := target.Kind != "website"
	if storeTarget {
		appendStage(run("appstore", func() model.Result {
			var result model.Result
			target, discovered, result = providers.Apps(ctx, s.get, target, discovered)
			return result
		}))
	}
	if target.URL != "" {
		appendStage(run("website", func() model.Result {
			var result model.Result
			var site model.Discovered
			target, site, result = collect.Website(ctx, s.get, target)
			discovered = combine(discovered, site)
			return result
		}))
	} else {
		appendStage(run("website", func() model.Result {
			return model.Result{Unknowns: []model.Unknown{{Channel: "website", Reason: "No public developer website was resolved from the store listing"}}}
		}))
	}
	wanted := func(a string) bool { return len(opts.Only) == 0 || slices.Contains(opts.Only, a) }
	siteFounders := discovered.Founders
	if wanted("founders") {
		appendStage(run("wikidata", func() model.Result {
			var result model.Result
			discovered, result = providers.FoundersFree(ctx, s.get, target, discovered)
			return result
		}))
	}
	for p, h := range opts.Handles {
		discovered.Socials[p] = h
	}
	type job struct {
		area, name string
		call       func() model.Result
	}
	jobs := []job{
		{"seo", "seo", func() model.Result { return collect.SEO(ctx, s.get, target) }},
		{"history", "wayback", func() model.Result { return providers.History(ctx, s.get, target) }},
		{"apps", "apple_ads", func() model.Result { return providers.AppleAds(ctx, s.get, target, discovered) }},
		{"apps", "hackernews", func() model.Result { return providers.Community(ctx, s.get, target) }},
	}
	if !storeTarget && wanted("apps") {
		appendStage(run("appstore", func() model.Result {
			var result model.Result
			_, discovered, result = providers.Apps(ctx, s.get, target, discovered)
			return result
		}))
	}
	slots := make([]stageResult, len(jobs))
	var wg sync.WaitGroup
	for i, j := range jobs {
		if wanted(j.area) && (target.URL != "" || j.name == "apple_ads") {
			wg.Go(func() { slots[i] = run(j.name, j.call) })
		}
	}
	// Paid collectors share a finite credit budget. Dispatch in declared priority order
	// so limited budgets produce repeatable results independently of request timing.
	for _, area := range []string{"paid", "social", "founders", "community"} {
		if !wanted(area) {
			continue
		}
		appendStage(run(area, func() model.Result {
			result := paid.Collect(ctx, area, target, discovered)
			if area == "founders" && len(siteFounders) > 0 {
				result.Evidence = append(result.Evidence, model.Evidence{Source: "founders", Kind: "observation", Title: "Founders named on public pages", URL: target.URL, Data: siteFounders})
			}
			return result
		}))
	}
	wg.Wait()
	for _, slot := range slots {
		if slot.source.Source != "" {
			appendStage(slot)
		}
	}
	if err := ctx.Err(); err != nil {
		return model.Bundle{}, err
	}
	b.Target = target
	b.Discovered = discovered
	b.Brand = first(discovered.BrandName, target.Brand, target.Domain, target.AppleID, target.PlayID, input)
	b.Credits.ScrapeCreators = paid.Spent()
	b.Identity = model.Identity{Logos: append([]model.Logo{}, discovered.Logos...), Founders: append([]model.Founder{}, discovered.Founders...)}
	if len(discovered.Logos) > 0 {
		b.Identity.Logo = discovered.Logos[0].URL
	}
	b.Scorecard = report.Score(b)
	progress("report", "start")
	md, err := s.WriteReport(ctx, b, opts.Engine, opts.Model)
	if err != nil {
		if ctx.Err() != nil {
			return model.Bundle{}, ctx.Err()
		}
		b.ReportWarning = err.Error()
		md = report.Markdown(b)
		progress("report", "error")
	} else {
		progress("report", "done")
	}
	if err = ctx.Err(); err != nil {
		return model.Bundle{}, err
	}
	return s.Store.Save(b, md)
}

func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func combine(a, b model.Discovered) model.Discovered {
	out := model.Discovered{
		Socials: map[string]string{}, BrandName: first(b.BrandName, a.BrandName),
		AppStoreIDs: unique(a.AppStoreIDs, b.AppStoreIDs), PlayIDs: unique(a.PlayIDs, b.PlayIDs),
	}
	for p, v := range a.Socials {
		out.Socials[p] = v
	}
	for p, v := range b.Socials {
		out.Socials[p] = v
	}
	for _, logo := range append(append([]model.Logo{}, b.Logos...), a.Logos...) {
		if !slices.ContainsFunc(out.Logos, func(existing model.Logo) bool { return existing.URL == logo.URL }) {
			out.Logos = append(out.Logos, logo)
		}
	}
	for _, founder := range append(append([]model.Founder{}, b.Founders...), a.Founders...) {
		if !slices.ContainsFunc(out.Founders, func(existing model.Founder) bool { return strings.EqualFold(existing.Name, founder.Name) }) {
			out.Founders = append(out.Founders, founder)
		}
	}
	return out
}

func unique(a, b []string) []string {
	out := append([]string{}, a...)
	for _, v := range b {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

func merge(b *model.Bundle, r model.Result, now time.Time) {
	refs := map[string]string{}
	for _, e := range r.Evidence {
		id := fmt.Sprintf("E%d", len(b.Evidence)+1)
		if e.ID != "" {
			refs[e.ID] = id
		}
		e.ID = id
		if e.ObservedAt == "" {
			e.ObservedAt = now.Format(time.RFC3339Nano)
		}
		if e.Kind == "" {
			e.Kind = "observation"
		}
		b.Evidence = append(b.Evidence, e)
	}
	for _, c := range r.Content {
		if c.ID == "" || slices.ContainsFunc(b.Content, func(old model.Content) bool { return old.Platform == c.Platform && old.ID == c.ID }) {
			continue
		}
		c.Evidence = refs[c.Evidence]
		b.Content = append(b.Content, c)
	}
	for _, u := range r.Unknowns {
		if !slices.ContainsFunc(b.Unknowns, func(old model.Unknown) bool { return old.Channel == u.Channel && old.Reason == u.Reason }) {
			b.Unknowns = append(b.Unknowns, u)
		}
	}
}
