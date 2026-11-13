package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"remgo/internal/model"
)

const (
	paidKeyMissing = "SCRAPECREATORS_API_KEY not set"
	paidAPI        = "https://api.scrapecreators.com"
	paidMaxBody    = 8 << 20
)

type Paid struct {
	client   *http.Client
	key      string
	budget   int
	mu       sync.Mutex
	spent    int
	cacheDir string
	now      func() time.Time
}

func NewPaid(client *http.Client, key string, maxCredits int) *Paid {
	if client == nil {
		client = http.DefaultClient
	}
	// The provider's API key must remain scoped to the configured origin.
	bound := *client
	bound.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Paid{client: &bound, key: strings.TrimSpace(key), budget: max(0, maxCredits), now: time.Now}
}

// Spent counts credit reservations. Requests that fail after dispatch still consume their reservation.
func (p *Paid) Spent() *int {
	if p == nil || p.key == "" {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	spent := p.spent
	return &spent
}

func (p *Paid) remaining() int {
	if p == nil || p.key == "" {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.budget - p.spent
}

func (p *Paid) get(ctx context.Context, path string, params url.Values) (paidObject, error) {
	if p == nil || p.key == "" {
		return nil, errors.New(paidKeyMissing)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()
	requestURL := paidAPI + path
	if len(params) > 0 {
		requestURL += "?" + params.Encode()
	}
	cachePath := p.cachePath(requestURL)
	if cached := p.readCache(cachePath); cached != nil {
		return cached, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, errors.New("Scrape Creators request could not be built")
	}
	req.Header.Set("x-api-key", p.key)
	req.Header.Set("Accept", "application/json")

	p.mu.Lock()
	if p.spent >= p.budget {
		p.mu.Unlock()
		return nil, fmt.Errorf("Scrape Creators credit budget reached (%d)", p.budget)
	}
	p.spent++
	p.mu.Unlock()
	response, err := p.client.Do(req)
	if err != nil {
		// Request errors can include URLs or sensitive headers on custom transports.
		return nil, fmt.Errorf("Scrape Creators request failed (%s)", path)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Scrape Creators HTTP %d (%s)", response.StatusCode, path)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, paidMaxBody+1))
	if err != nil || len(data) > paidMaxBody {
		return nil, fmt.Errorf("Scrape Creators response unavailable or too large (%s)", path)
	}
	body := paidDecode(data)
	if body == nil {
		return nil, fmt.Errorf("Scrape Creators returned invalid JSON (%s)", path)
	}
	if paidRejected(body) {
		return nil, fmt.Errorf("Scrape Creators rejected the request (%s)", path)
	}
	if paidHasSecret(body, p.key) {
		return nil, fmt.Errorf("Scrape Creators returned unexpected credential data (%s)", path)
	}
	delete(body, "credits_remaining")
	delete(body, "credits_charged")
	p.writeCache(cachePath, body)
	return body, nil
}

func (p *Paid) Collect(ctx context.Context, area string, target model.Target, d model.Discovered) model.Result {
	brand := paidFirst(d.BrandName, target.Brand, target.Domain)
	var result model.Result
	switch area {
	case "paid":
		p.ads(ctx, &result, target, d, brand)
	case "social":
		p.social(ctx, &result, target, d, brand)
	case "community":
		p.paidCommunity(ctx, &result, target, d, brand)
	case "founders":
		p.founders(ctx, &result, d, brand)
	}
	return result
}

type paidObject map[string]any

func paidObj(x any) paidObject {
	switch v := x.(type) {
	case paidObject:
		return v
	case map[string]any:
		return paidObject(v)
	default:
		return nil
	}
}

func paidItems(x any) []any {
	v, _ := x.([]any)
	return v
}

func paidAt(x any, fields ...string) any {
	for _, key := range fields {
		x = paidObj(x)[key]
	}
	return x
}

func paidStr(x any) string {
	switch v := x.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return string(v)
	default:
		return ""
	}
}

func paidFirst(values ...string) string {
	for _, x := range values {
		if x != "" {
			return x
		}
	}
	return ""
}

func paidNum(x any) (float64, bool) {
	v, err := strconv.ParseFloat(strings.ReplaceAll(paidStr(x), ",", ""), 64)
	return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func paidValue(x any) any {
	if n, ok := paidNum(x); ok {
		return n
	}
	return nil
}

func paidMetrics(values map[string]any) map[string]float64 {
	result := make(map[string]float64)
	for key, value := range values {
		if n, ok := paidNum(value); ok && n >= 0 {
			result[key] = n
		}
	}
	return result
}

func paidClip(s string, maxRunes int) string {
	value := []rune(strings.Join(strings.Fields(s), " "))
	return string(value[:min(len(value), maxRunes)])
}

func paidNorm(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}

func paidQuery(base string, params url.Values) string {
	return base + "?" + params.Encode()
}

func paidURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return ""
	}
	return u.String()
}

func paidOnSite(raw, domain, pathPrefix string) *url.URL {
	u, err := url.Parse(paidURL(raw))
	if err != nil || !(strings.EqualFold(u.Hostname(), domain) || strings.EqualFold(u.Hostname(), "www."+domain)) ||
		!strings.HasPrefix(u.EscapedPath(), pathPrefix) {
		return nil
	}
	return u
}

func paidUnknown(result *model.Result, channel, manual string, err error) {
	result.Unknowns = append(result.Unknowns, model.Unknown{Channel: channel, Reason: err.Error(), ManualCheck: manual})
}

func paidObservation(result *model.Result, source, kind, title, link string, data any, content []model.Content) {
	id := fmt.Sprintf("paid-%d", len(result.Evidence)+1)
	result.Evidence = append(result.Evidence, model.Evidence{ID: id, Source: source, Kind: kind, Title: title, URL: link, Data: data})
	seen := make(map[string]bool, len(result.Content)+len(content))
	for _, c := range result.Content {
		seen[c.ID] = true
	}
	for _, c := range content {
		if c.ID != "" && c.URL != "" && !seen[c.ID] {
			c.Evidence = id
			result.Content = append(result.Content, c)
			seen[c.ID] = true
		}
	}
}

func paidArray(body paidObject, key string) ([]any, error) {
	a, ok := body[key].([]any)
	if !ok {
		return nil, fmt.Errorf("Scrape Creators response missing %s array", key)
	}
	return a, nil
}

func paidFetchArray(ctx context.Context, p *Paid, path string, params url.Values, field string) (paidObject, []any, error) {
	body, err := p.get(ctx, path, params)
	if err != nil {
		return nil, nil, err
	}
	rows, err := paidArray(body, field)
	return body, rows, err
}

func paidDate(raw any) time.Time {
	if n, ok := paidNum(raw); ok && n > 0 && n < 253402300800 {
		return time.Unix(int64(n), 0).UTC()
	}
	s := paidStr(raw)
	for _, layout := range []string{time.RFC3339Nano, time.DateOnly, time.RubyDate, time.RFC1123Z} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func paidDateText(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.DateOnly)
}
