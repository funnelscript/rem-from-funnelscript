package collect

import (
	"regexp"
	"strings"

	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

var (
	agitationPattern   = regexp.MustCompile(`(?i)\b(tired of|struggling|waste|wasting|stop doing|frustrated|painful|broken|hard way|nightmare|hate|mess|chaos)\b`)
	socialProofPattern = regexp.MustCompile(`(?i)\b(trusted by|used by|teams switch|rated 5 stars|reviews|customers love|featured in|over \d+|join \d+)\b`)
	shockStatPattern   = regexp.MustCompile(`(?i)\b(\d+x|\d+%\s*(faster|reduction|increase|more|less)|\$\d+[km]?\+?|\d+\s*(hours|days)\s*saved)\b`)
	founderPattern     = regexp.MustCompile(`(?i)\b(why we built|i started|our mission|bootstrapped|behind the scenes|my journey|we created)\b`)
	usVsThemPattern    = regexp.MustCompile(`(?i)\b(alternative to|vs\b|better than|switch from|replace|compared to|unlike)\b`)
	directOfferPattern = regexp.MustCompile(`(?i)\b(free trial|get started free|discount|special offer|no credit card|30-day|money-back|free forever)\b`)
)

// DeconstructCreative decomposes collected content items and website positioning into REA-structured creative angles.
func DeconstructCreative(contents []model.Content, evidences []model.Evidence) []model.CreativeAngle {
	var angles []model.CreativeAngle

	// 1. Deconstruct paid & organic content items
	for _, c := range contents {
		text := strings.TrimSpace(c.Text)
		if text == "" {
			text = strings.TrimSpace(c.Title)
		}
		if text == "" {
			continue
		}

		angle := classifyAngle(text, c.Kind, c.Evidence)
		if angle.Hook != "" {
			angles = append(angles, angle)
		}
	}

	// 2. Deconstruct key website positioning evidence if content items are sparse
	for _, e := range evidences {
		if e.Source == "website" && e.Kind == "observation" {
			if data, ok := e.Data.(map[string]any); ok {
				var headlines []string
				if title, ok := data["title"].(string); ok && title != "" {
					headlines = append(headlines, title)
				}
				if h1List, ok := data["h1"].([]string); ok {
					headlines = append(headlines, h1List...)
				}
				for _, h := range headlines {
					if angle := classifyAngle(h, "website_headline", e.ID); angle.Hook != "" {
						angles = append(angles, angle)
					}
				}
			}
		}
	}

	return deduplicateAngles(angles)
}

func classifyAngle(text, format, evidenceID string) model.CreativeAngle {
	text = strings.TrimSpace(text)
	lines := strings.Split(text, "\n")
	hook := strings.TrimSpace(lines[0])
	if len([]rune(hook)) > 140 {
		hook = string([]rune(hook)[:137]) + "..."
	}

	category := "general_value"
	switch {
	case agitationPattern.MatchString(text):
		category = "agitation"
	case usVsThemPattern.MatchString(text):
		category = "us_vs_them"
	case shockStatPattern.MatchString(text):
		category = "shock_stat"
	case socialProofPattern.MatchString(text):
		category = "social_proof"
	case founderPattern.MatchString(text):
		category = "founder_story"
	case directOfferPattern.MatchString(text):
		category = "direct_offer"
	}

	var mechanism string
	if len(lines) > 1 {
		mechanism = strings.TrimSpace(lines[1])
	} else {
		mechanism = "Direct solution proposition"
	}

	var proof string
	if match := socialProofPattern.FindString(text); match != "" {
		proof = match
	} else if match := shockStatPattern.FindString(text); match != "" {
		proof = match
	}

	var offerCTA string
	if match := directOfferPattern.FindString(text); match != "" {
		offerCTA = match
	}

	var evIDs []string
	if evidenceID != "" {
		evIDs = append(evIDs, evidenceID)
	}

	return model.CreativeAngle{
		Category:    category,
		Hook:        hook,
		Mechanism:   mechanism,
		Proof:       proof,
		OfferCTA:    offerCTA,
		Format:      normalizeFormat(format),
		EvidenceIDs: evIDs,
	}
}

func normalizeFormat(fmtStr string) string {
	f := strings.ToLower(fmtStr)
	switch {
	case strings.Contains(f, "video") || strings.Contains(f, "reel") || strings.Contains(f, "tiktok"):
		return "video"
	case strings.Contains(f, "carousel"):
		return "carousel"
	case strings.Contains(f, "image"):
		return "static_image"
	case strings.Contains(f, "headline"):
		return "landing_page_hero"
	default:
		return "copy_post"
	}
}

func deduplicateAngles(angles []model.CreativeAngle) []model.CreativeAngle {
	seen := make(map[string]bool)
	var result []model.CreativeAngle
	for _, a := range angles {
		key := a.Category + ":" + strings.ToLower(a.Hook)
		if !seen[key] {
			seen[key] = true
			result = append(result, a)
		}
	}
	return result
}
