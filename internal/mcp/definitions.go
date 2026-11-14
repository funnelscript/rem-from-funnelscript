package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/funnelscript/rem-from-funnelscript/internal/app"
)

var (
	platforms  = []string{"meta_ads", "google_ads", "linkedin_ads", "apple_ads", "tiktok", "instagram", "youtube", "x", "linkedin", "reddit", "hackernews", "producthunt"}
	sortOrders = []string{"top", "newest", "longest_running"}
)

type schema map[string]any

func stringField(description string) schema {
	return schema{"type": "string", "description": description}
}

func stringEnum(description string, values []string) schema {
	return schema{"type": "string", "description": description, "enum": values}
}

func integerField(description string, minimum, maximum int) schema {
	return schema{"type": "integer", "description": description, "minimum": minimum, "maximum": maximum}
}

func object(properties map[string]schema, required ...string) schema {
	result := schema{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) != 0 {
		result["required"] = required
	}
	return result
}

type toolDefinition struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	InputSchema schema `json:"inputSchema"`
}

var tools = []toolDefinition{
	{
		Name:        "reverse_engineer_marketing",
		Title:       "Investigate a company's marketing",
		Description: "Collect public marketing evidence for a website, App Store or Google Play URL. Returns a saved run ID, channel scorecard, evidence index and unknowns. Use get_evidence and get_content for the underlying material.",
		InputSchema: object(map[string]schema{
			"url":         stringField("Website domain, website URL or app store URL."),
			"only":        {"type": "array", "description": "Areas to investigate. Website collection always runs when a website is available.", "items": stringEnum("Marketing area", app.Areas)},
			"handles":     {"type": "object", "description": "Override discovered handles, such as x or tiktok.", "additionalProperties": schema{"type": "string"}},
			"max_credits": integerField("Maximum paid Scrape Creators credits for this run. Set to zero to skip paid requests.", 0, 500),
			"stealth":     {"type": "boolean", "description": "Enable Scrapling-style browser TLS/HTTP-2 fingerprinting & anti-bot bypass."},
			"keyless":     {"type": "boolean", "description": "Rely entirely on zero-cost extractors without consuming paid credits."},
			"window":      stringField("Recency filter window (e.g. 30d, 90d; default: 30d)."),
		}, "url"),
	},
	{
		Name:        "get_evidence",
		Title:       "Get evidence details",
		Description: "Read full evidence records from a saved run, optionally filtered by IDs or collector source.",
		InputSchema: object(map[string]schema{
			"run_id":           stringField("ID returned by reverse_engineer_marketing."),
			"ids":              {"type": "array", "description": "Evidence IDs, such as E3 and E7. Omit for all evidence.", "items": schema{"type": "string"}},
			"source":           stringField("Collector source, such as seo or meta_ads."),
			"include_unknowns": {"type": "boolean", "description": "Return unknowns with evidence, including reasons and any manual check links."},
		}, "run_id"),
	},
	{
		Name:        "get_content",
		Title:       "Get posts, videos and ads",
		Description: "Read ads, posts, videos and threads collected in a run, including original URLs, evidence references and available metrics.",
		InputSchema: object(map[string]schema{
			"run_id":   stringField("ID returned by reverse_engineer_marketing."),
			"platform": stringEnum("Platform filter.", platforms),
			"sort":     stringEnum("Sort order. Omit for collector order.", sortOrders),
			"limit":    integerField("Maximum items to return. Defaults to 30.", 1, 100),
		}, "run_id"),
	},
	{
		Name:        "get_playbook",
		Title:       "Get the analysis playbook",
		Description: "Read the analysis method and report template, or a named channel guide.",
		InputSchema: object(map[string]schema{
			"name": stringField("Examples: paid-meta, paid-search, paid-b2b, organic-short-form, founder-led, seo-geo, community-launches, funnel-monetization, apps, reddit."),
		}),
	},
	{
		Name:        "validate_report",
		Title:       "Validate a report",
		Description: "Check a Markdown report for evidence references, grounding, required sections and unsupported spending claims.",
		InputSchema: object(map[string]schema{
			"run_id":   stringField("ID of the evidence run cited in the report."),
			"markdown": stringField("Full Markdown to validate."),
		}, "run_id", "markdown"),
	},
	{
		Name:        "write_report",
		Title:       "Write a report",
		Description: "Render a saved run as a Markdown report. The default uses Anthropic when configured and otherwise renders the deterministic evidence report.",
		InputSchema: object(map[string]schema{
			"run_id": stringField("ID of the evidence run to report."),
			"model":  stringField("Optional report writer model."),
			"engine": stringField("Report engine selection. Defaults to auto."),
		}, "run_id"),
	},
	{
		Name:        "compare_runs",
		Title:       "Compare saved runs",
		Description: "Compare two snapshots of the same company by run ID or use the two latest saved runs for a domain.",
		InputSchema: object(map[string]schema{
			"domain": stringField("Website domain for comparison of the two newest runs."),
			"run_a":  stringField("ID of the earlier run when comparing explicit snapshots."),
			"run_b":  stringField("ID of the later run when comparing explicit snapshots."),
		}),
	},
}

type promptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type promptDefinition struct {
	Name        string           `json:"name"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Arguments   []promptArgument `json:"arguments"`
	render      func(map[string]string) string
}

var urlArgument = promptArgument{Name: "url", Description: "Website domain, website URL or app store URL.", Required: true}

var prompts = []promptDefinition{
	{
		Name:        "investigate_company",
		Title:       "Investigate a company",
		Description: "Collect evidence, write a cited analysis and validate the draft.",
		Arguments:   []promptArgument{urlArgument},
		render: func(args map[string]string) string {
			return fmt.Sprintf("Investigate how the company at %s acquires customers. Start with reverse_engineer_marketing, then use get_playbook for the report format and get_evidence or get_content to retrieve what you cite. Cite observations with [E#], label inferences and keep gaps explicit. Call validate_report with the run ID and revise the draft before delivery.", quoted(args["url"]))
		},
	},
	{
		Name:        "compare_competitors",
		Title:       "Compare competitors",
		Description: "Compare the marketing of two to four companies.",
		Arguments:   []promptArgument{{Name: "urls", Description: "Comma-separated list of two to four domains.", Required: true}},
		render: func(args map[string]string) string {
			return fmt.Sprintf("Compare these companies: %s. Run reverse_engineer_marketing for each. Build a channel table, describe each company's acquisition strategy, identify gaps and propose one tactic to adapt from each. Prefix evidence IDs with the company name so each citation resolves to the right run.", quoted(args["urls"]))
		},
	},
	{
		Name:        "what_changed",
		Title:       "What changed since the previous run",
		Description: "Investigate a company again and compare the latest saved snapshots.",
		Arguments:   []promptArgument{urlArgument},
		render: func(args map[string]string) string {
			return fmt.Sprintf("Investigate %s again with reverse_engineer_marketing. Call compare_runs with the investigated domain or the two saved run IDs. Explain measured changes in positioning, ads, audiences or SEO where available. Separate observed changes from observations the second run could not repeat.", quoted(args["url"]))
		},
	},
	{
		Name:        "steal_the_playbook",
		Title:       "Adapt a marketing playbook",
		Description: "Turn a cited competitor investigation into a 30-day plan for your product.",
		Arguments:   []promptArgument{urlArgument, {Name: "my_product", Description: "The product the plan should serve."}},
		render: func(args map[string]string) string {
			product := args["my_product"]
			if product == "" {
				product = "my product"
			}
			return fmt.Sprintf("Investigate %s and cite the channels, offers, funnel and creative patterns that are observed. For %s, draft a 30-day plan with 10 original hooks, 3 creator briefs, a landing-page outline, a posting schedule and an SEO page template. Use original copy and media. State the measures to track and when to compare the next runs.", quoted(args["url"]), quoted(product))
		},
	},
	{
		Name:        "audit_unknowns",
		Title:       "Audit unknowns",
		Description: "Review evidence gaps in a saved run and identify ways to check them.",
		Arguments:   []promptArgument{{Name: "run_id", Description: "ID returned by reverse_engineer_marketing.", Required: true}},
		render: func(args map[string]string) string {
			return fmt.Sprintf("Call get_evidence with run_id %s and include_unknowns true. Explain why each gap exists, which claims it prevents, and how to check the most important gaps using available public sources or the recorded manualCheck links.", quoted(args["run_id"]))
		},
	},
}

func getPrompt(raw json.RawMessage) (any, *rpcError) {
	var request struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
		Meta      map[string]any    `json:"_meta"`
	}
	if err := decodeObject(raw, &request); err != nil {
		return nil, invalidParams("A prompt name and optional string arguments are required")
	}
	for _, prompt := range prompts {
		if prompt.Name != request.Name {
			continue
		}
		allowed := make(map[string]bool, len(prompt.Arguments))
		for _, argument := range prompt.Arguments {
			allowed[argument.Name] = true
			if argument.Required && strings.TrimSpace(request.Arguments[argument.Name]) == "" {
				return nil, invalidParams("Missing required prompt argument: " + argument.Name)
			}
		}
		for name := range request.Arguments {
			if !allowed[name] {
				return nil, invalidParams("Unknown prompt argument: " + name)
			}
		}
		message := map[string]any{
			"role":    "user",
			"content": textContent{Type: "text", Text: prompt.render(request.Arguments)},
		}
		return map[string]any{"description": prompt.Description, "messages": []any{message}}, nil
	}
	return nil, invalidParams("Unknown prompt: " + request.Name)
}

func quoted(s string) []byte {
	value, _ := json.Marshal(s)
	return value
}
