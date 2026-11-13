package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/funnelscript/rem-from-funnelscript/internal/app"
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
	"github.com/funnelscript/rem-from-funnelscript/internal/report"
)

// Serve runs the MCP server over newline-delimited JSON-RPC on stdio.
func Serve(ctx context.Context, in io.Reader, out io.Writer, service *app.Service) error {
	if service == nil || service.Store == nil {
		return errors.New("MCP requires a marketing service and run store")
	}
	server := marketing{service: &appAdapter{service}}
	return serveProtocol(ctx, in, out, model.Version, server.handle)
}

type marketingService interface {
	Investigate(context.Context, string, app.Options) (model.Bundle, error)
	WriteReport(context.Context, model.Bundle, string, string) (string, error)
	load(string) (model.Bundle, error)
	latestPair(string) (model.Bundle, model.Bundle, error)
}

type appAdapter struct{ *app.Service }

func (a *appAdapter) load(id string) (model.Bundle, error) { return a.Store.Load(id) }
func (a *appAdapter) latestPair(domain string) (model.Bundle, model.Bundle, error) {
	return a.Store.LatestPair(domain)
}

type marketing struct{ service marketingService }

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content []textContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

func (m marketing) handle(ctx context.Context, method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "tools/list":
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		return m.call(ctx, params)
	case "prompts/list":
		return map[string]any{"prompts": prompts}, nil
	case "prompts/get":
		return getPrompt(params)
	default:
		return nil, &rpcError{-32601, "Method not found"}
	}
}

func (m marketing) call(ctx context.Context, raw json.RawMessage) (any, *rpcError) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
		Meta      map[string]any  `json:"_meta"`
	}
	if err := decodeObject(raw, &call); err != nil || strings.TrimSpace(call.Name) == "" {
		return nil, invalidParams("A tool name and optional arguments object are required")
	}
	if len(call.Arguments) != 0 && call.Arguments[0] != '{' {
		return nil, invalidParams("Tool arguments must be an object")
	}
	var invoke func(context.Context, json.RawMessage) (string, error)
	switch call.Name {
	case "reverse_engineer_marketing":
		invoke = m.investigate
	case "get_evidence":
		invoke = m.getEvidence
	case "get_content":
		invoke = m.getContent
	case "get_playbook":
		invoke = getPlaybook
	case "validate_report":
		invoke = m.validateReport
	case "write_report":
		invoke = m.writeReport
	case "compare_runs":
		invoke = m.compareRuns
	default:
		return nil, invalidParams("Unknown tool: " + call.Name)
	}
	value, err := invoke(ctx, call.Arguments)
	if err != nil {
		value = err.Error()
	}
	return toolResult{Content: []textContent{{Type: "text", Text: value}}, IsError: err != nil}, nil
}

func decodeObject(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	if raw[0] != '{' {
		return errors.New("expected an arguments object")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func (m marketing) investigate(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		URL        string            `json:"url"`
		Only       []string          `json:"only"`
		Handles    map[string]string `json:"handles"`
		MaxCredits *int              `json:"max_credits"`
	}
	if err := decodeObject(raw, &args); err != nil {
		return "", err
	}
	if strings.TrimSpace(args.URL) == "" {
		return "", errors.New("url is required")
	}
	for _, area := range args.Only {
		if !slices.Contains(app.Areas, area) {
			return "", fmt.Errorf("unknown area %q", area)
		}
	}
	opts := app.Options{Only: args.Only, Handles: args.Handles, Engine: "evidence"}
	if args.MaxCredits != nil {
		if *args.MaxCredits < 0 || *args.MaxCredits > 500 {
			return "", errors.New("max_credits must be between 0 and 500")
		}
		opts.MaxCredits, opts.BudgetSet = *args.MaxCredits, true
	}
	bundle, err := m.service.Investigate(ctx, args.URL, opts)
	if err != nil {
		return "", err
	}
	return investigationSummary(bundle), nil
}

func investigationSummary(bundle model.Bundle) string {
	var summary strings.Builder
	fmt.Fprintf(&summary, "# %s: evidence collected (run_id: %s)\n", report.Clean(oneLine(bundle.Brand)), bundle.ID)
	fmt.Fprintf(&summary, "%d evidence items, %d posts/videos/ads, %d unknowns.\n", len(bundle.Evidence), len(bundle.Content), len(bundle.Unknowns))
	if bundle.Credits.ScrapeCreators != nil {
		fmt.Fprintf(&summary, "Scrape Creators credits: %d.\n", *bundle.Credits.ScrapeCreators)
	}
	if bundle.Harness.Version != "" {
		fmt.Fprintf(&summary, "Harness %s (%s).\n", bundle.Harness.Version, bundle.Harness.SHA)
	}
	summary.WriteString("\n## Channel scorecard\n")
	for _, score := range bundle.Scorecard {
		fmt.Fprintf(&summary, "- %s: %s. %s", report.Clean(score.Channel), report.Clean(score.Status), report.Clean(oneLine(score.Signal)))
		for _, id := range score.EvidenceIDs {
			fmt.Fprintf(&summary, " [%s]", id)
		}
		summary.WriteByte('\n')
	}
	summary.WriteString("\n## Evidence index\n")
	for _, item := range bundle.Evidence {
		fmt.Fprintf(&summary, "- [%s] (%s) %s", item.ID, report.Clean(item.Source), report.Clean(oneLine(item.Title)))
		if link := report.SafeURL(item.URL); link != "" {
			fmt.Fprintf(&summary, " (%s)", link)
		}
		summary.WriteByte('\n')
	}
	summary.WriteString("\n## Unknowns\n")
	if len(bundle.Unknowns) == 0 {
		summary.WriteString("- No unknowns recorded for this run.\n")
	}
	for _, unknown := range bundle.Unknowns {
		fmt.Fprintf(&summary, "- %s: %s", report.Clean(unknown.Channel), report.Clean(oneLine(unknown.Reason)))
		if link := report.SafeURL(unknown.ManualCheck); link != "" {
			fmt.Fprintf(&summary, " (%s)", link)
		}
		summary.WriteByte('\n')
	}
	summary.WriteString("\nRead get_playbook, retrieve supporting items with get_evidence or get_content, cite their [E#] identifiers, then call validate_report.")
	return summary.String()
}

func oneLine(value string) string { return strings.Join(strings.Fields(value), " ") }

func (m marketing) run(id string) (model.Bundle, error) {
	if strings.TrimSpace(id) == "" {
		return model.Bundle{}, errors.New("run_id is required")
	}
	bundle, err := m.service.load(id)
	if err != nil {
		return model.Bundle{}, fmt.Errorf("could not load run %q: %w", id, err)
	}
	return bundle, nil
}

func (m marketing) getEvidence(_ context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		RunID           string   `json:"run_id"`
		IDs             []string `json:"ids"`
		Source          string   `json:"source"`
		IncludeUnknowns bool     `json:"include_unknowns"`
	}
	if err := decodeObject(raw, &args); err != nil {
		return "", err
	}
	bundle, err := m.run(args.RunID)
	if err != nil {
		return "", err
	}
	evidence := make([]model.Evidence, 0, len(bundle.Evidence))
	for _, item := range bundle.Evidence {
		if (len(args.IDs) == 0 || slices.Contains(args.IDs, item.ID)) && (args.Source == "" || args.Source == item.Source) {
			evidence = append(evidence, item)
		}
	}
	if args.IncludeUnknowns {
		unknowns := bundle.Unknowns
		if unknowns == nil {
			unknowns = []model.Unknown{}
		}
		return jsonText(map[string]any{"evidence": evidence, "unknowns": unknowns})
	}
	return jsonText(evidence)
}

func (m marketing) getContent(_ context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		RunID    string `json:"run_id"`
		Platform string `json:"platform"`
		Sort     string `json:"sort"`
		Limit    *int   `json:"limit"`
	}
	if err := decodeObject(raw, &args); err != nil {
		return "", err
	}
	if args.Platform != "" && !slices.Contains(platforms, args.Platform) {
		return "", fmt.Errorf("unknown platform %q", args.Platform)
	}
	if args.Sort != "" && !slices.Contains(sortOrders, args.Sort) {
		return "", fmt.Errorf("unknown sort order %q", args.Sort)
	}
	limit := 30
	if args.Limit != nil {
		limit = *args.Limit
		if limit < 1 || limit > 100 {
			return "", errors.New("limit must be between 1 and 100")
		}
	}
	bundle, err := m.run(args.RunID)
	if err != nil {
		return "", err
	}
	items := make([]model.Content, 0, len(bundle.Content))
	for _, item := range bundle.Content {
		if args.Platform == "" || args.Platform == item.Platform {
			items = append(items, item)
		}
	}
	switch args.Sort {
	case "newest":
		sort.SliceStable(items, func(i, j int) bool { return items[i].Date > items[j].Date })
	case "longest_running":
		sort.SliceStable(items, func(i, j int) bool { return items[i].Metrics["runningDays"] > items[j].Metrics["runningDays"] })
	case "top":
		sort.SliceStable(items, func(i, j int) bool { return topScore(items[i]) > topScore(items[j]) })
	}
	return jsonText(items[:min(limit, len(items))])
}

func topScore(item model.Content) float64 {
	for _, key := range []string{"views", "likes", "score", "runningDays"} {
		if value, exists := item.Metrics[key]; exists {
			return value
		}
	}
	return 0
}

func getPlaybook(_ context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Name string `json:"name"`
	}
	if err := decodeObject(raw, &args); err != nil {
		return "", err
	}
	if args.Name == "" {
		return report.Playbook(), nil
	}
	value := report.NamedPlaybook(args.Name)
	if value == "" {
		return "", fmt.Errorf("unknown playbook %q", args.Name)
	}
	return value, nil
}

func (m marketing) validateReport(_ context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		RunID    string `json:"run_id"`
		Markdown string `json:"markdown"`
	}
	if err := decodeObject(raw, &args); err != nil {
		return "", err
	}
	if strings.TrimSpace(args.Markdown) == "" {
		return "", errors.New("markdown is required")
	}
	bundle, err := m.run(args.RunID)
	if err != nil {
		return "", err
	}
	return jsonText(report.Validate(args.Markdown, bundle))
}

func (m marketing) writeReport(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		RunID  string `json:"run_id"`
		Model  string `json:"model"`
		Engine string `json:"engine"`
	}
	if err := decodeObject(raw, &args); err != nil {
		return "", err
	}
	bundle, err := m.run(args.RunID)
	if err != nil {
		return "", err
	}
	if args.Engine == "" {
		args.Engine = "auto"
	}
	return m.service.WriteReport(ctx, bundle, args.Engine, args.Model)
}

func (m marketing) compareRuns(_ context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Domain string `json:"domain"`
		RunA   string `json:"run_a"`
		RunB   string `json:"run_b"`
	}
	if err := decodeObject(raw, &args); err != nil {
		return "", err
	}
	if args.Domain != "" && args.RunA == "" && args.RunB == "" {
		target, err := app.ParseTarget(args.Domain)
		if err != nil || target.Domain == "" {
			return "", errors.New("domain must be a website domain or URL")
		}
		before, after, err := m.service.latestPair(target.Domain)
		if err != nil {
			return "", err
		}
		return report.RenderDiff(report.Compare(before, after)), nil
	} else if args.Domain != "" || args.RunA == "" || args.RunB == "" {
		return "", errors.New("supply either domain or both run_a and run_b")
	}
	before, err := m.run(args.RunA)
	if err != nil {
		return "", err
	}
	after, err := m.run(args.RunB)
	if err != nil {
		return "", err
	}
	if !sameTarget(before.Target, after.Target) {
		return "", errors.New("the runs belong to different targets")
	}
	return report.RenderDiff(report.Compare(before, after)), nil
}

func sameTarget(a, b model.Target) bool {
	if a.Domain != "" && b.Domain != "" {
		return a.Domain == b.Domain || slices.Contains(a.Aliases, b.Domain) || slices.Contains(b.Aliases, a.Domain)
	}
	return a.AppleID != "" && a.AppleID == b.AppleID ||
		a.PlayID != "" && a.PlayID == b.PlayID ||
		a.Input != "" && a.Input == b.Input
}

func jsonText(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
