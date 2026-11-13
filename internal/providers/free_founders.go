package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/funnelscript/rem-from-funnelscript/internal/fetch"
	"github.com/funnelscript/rem-from-funnelscript/internal/model"
)

const (
	wikidataAPI  = "https://www.wikidata.org/w/api.php?"
	wikidataItem = "https://www.wikidata.org/wiki/"
)

var wikidataID = regexp.MustCompile(`^Q[1-9][0-9]{0,15}$`)

type wikidataEntity struct {
	Labels       map[string]struct{ Value string } `json:"labels"`
	Descriptions map[string]struct{ Value string } `json:"descriptions"`
	Claims       map[string][]struct {
		Rank     string `json:"rank"`
		MainSnak struct {
			DataValue struct{ Value json.RawMessage } `json:"datavalue"`
		} `json:"mainsnak"`
	} `json:"claims"`
}

func wikidataValues[T any](entity wikidataEntity, prop string) []T {
	var values []T
	for _, claim := range entity.Claims[prop] {
		if claim.Rank == "deprecated" || len(claim.MainSnak.DataValue.Value) == 0 {
			continue
		}
		var value T
		if err := json.Unmarshal(claim.MainSnak.DataValue.Value, &value); err == nil {
			values = append(values, value)
		}
	}
	return values
}

func wikidataIDs(entity wikidataEntity, properties ...string) []string {
	var out []string
	for _, property := range properties {
		for _, value := range wikidataValues[struct{ ID string }](entity, property) {
			if wikidataID.MatchString(value.ID) && !slices.Contains(out, value.ID) {
				out = append(out, value.ID)
			}
		}
	}
	return out
}

func getWikidataEntities(ctx context.Context, get fetch.Getter, ids []string) (map[string]wikidataEntity, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := url.Values{"action": {"wbgetentities"}, "ids": {strings.Join(ids, "|")}, "props": {"claims|labels|descriptions"}, "languages": {"en"}, "format": {"json"}}
	var result struct {
		Entities map[string]wikidataEntity `json:"entities"`
	}
	if err := freeJSON(ctx, get, wikidataAPI+query.Encode(), &result); err != nil {
		return nil, err
	}
	if result.Entities == nil {
		return nil, errors.New("missing Wikidata entities")
	}
	return result.Entities, nil
}

// FoundersFree follows Wikidata founder claims only after matching the official website.
func FoundersFree(ctx context.Context, get fetch.Getter, target model.Target, discovered model.Discovered) (model.Discovered, model.Result) {
	var result model.Result
	brand := discovered.BrandName
	if brand == "" {
		brand = target.Brand
	}
	if brand == "" || target.Domain == "" {
		appendUnknown(&result, "founder_led", "Wikidata founder lookup needs a brand name and website domain", wikidataItem+"Wikidata:Main_Page")
		return discovered, result
	}
	brand = brief(brand, 120)
	manual := wikidataItem + "Special:Search?" + url.Values{"search": {brand}}.Encode()
	query := url.Values{"action": {"wbsearchentities"}, "search": {brand}, "language": {"en"}, "format": {"json"}, "limit": {"8"}, "type": {"item"}}
	var response struct {
		Search []struct{ ID string } `json:"search"`
	}
	if err := freeJSON(ctx, get, wikidataAPI+query.Encode(), &response); err != nil {
		appendUnknown(&result, "founder_led", "Wikidata search unavailable: "+err.Error(), manual)
		return discovered, result
	}
	if response.Search == nil {
		appendUnknown(&result, "founder_led", "Wikidata returned an unrecognized search response", manual)
		return discovered, result
	}
	var ids []string
	for _, candidate := range response.Search {
		if wikidataID.MatchString(candidate.ID) && !slices.Contains(ids, candidate.ID) && len(ids) < 8 {
			ids = append(ids, candidate.ID)
		}
	}
	entities, err := getWikidataEntities(ctx, get, ids)
	if err != nil {
		appendUnknown(&result, "founder_led", "Wikidata entities unavailable: "+err.Error(), manual)
		return discovered, result
	}
	var matchID string
	for _, id := range ids {
		if wikidataMatchesTarget(entities[id], target) {
			matchID = id
			break
		}
	}
	if matchID == "" {
		appendUnknown(&result, "founder_led", "Wikidata search returned no item with a matching official website", manual)
		return discovered, result
	}
	matched := entities[matchID]
	orgID, org := matchID, matched
	if len(wikidataIDs(matched, "P112")) == 0 {
		parents := wikidataIDs(matched, "P178", "P127", "P749")
		parents = parents[:min(len(parents), 3)]
		related, err := getWikidataEntities(ctx, get, parents)
		if err != nil {
			appendUnknown(&result, "founder_led", "Wikidata developer or owner claims unavailable: "+err.Error(), wikidataItem+matchID)
		}
		for _, id := range parents {
			if len(wikidataIDs(related[id], "P112")) > 0 {
				orgID, org = id, related[id]
				break
			}
		}
	}
	itemURL := wikidataItem + orgID
	founderIDs := wikidataIDs(org, "P112")
	founderIDs = founderIDs[:min(len(founderIDs), 10)]
	founders := []model.Founder{}
	people, err := getWikidataEntities(ctx, get, founderIDs)
	if err != nil {
		appendUnknown(&result, "founder_led", "Wikidata founder names unavailable: "+err.Error(), itemURL)
	}
	for _, id := range founderIDs {
		if name := brief(people[id].Labels["en"].Value, 100); name != "" {
			founders = append(founders, model.Founder{Name: name, Role: "Founder", URL: wikidataItem + id})
		}
	}
	if err == nil && len(founders) == 0 {
		appendUnknown(&result, "founder_led", "No founder name was available from the matched Wikidata item", itemURL)
	}
	for _, founder := range founders {
		if !slices.ContainsFunc(discovered.Founders, func(existing model.Founder) bool {
			return strings.EqualFold(existing.Name, founder.Name) && existing.URL == founder.URL
		}) {
			discovered.Founders = append(discovered.Founders, founder)
		}
	}
	logo := wikidataLogo(matched, org)
	if logo != "" && !slices.ContainsFunc(discovered.Logos, func(existing model.Logo) bool { return existing.URL == logo }) {
		discovered.Logos = append(discovered.Logos, model.Logo{URL: logo, Source: "wikidata"})
	}
	name := brief(org.Labels["en"].Value, 100)
	if name == "" {
		name = brand
	}
	data := map[string]any{"item": matchID, "organization": orgID, "name": name, "description": brief(org.Descriptions["en"].Value, 180), "founders": founders}
	if logo != "" {
		data["logo"] = logo
	}
	if dates := wikidataValues[struct{ Time string }](org, "P571"); len(dates) > 0 && strings.HasPrefix(dates[0].Time, "+") && len(dates[0].Time) >= 6 {
		data["inception"] = dates[0].Time[1:5]
	}
	title := "Wikidata: official website matches " + name
	if len(founders) > 0 {
		title = fmt.Sprintf("Wikidata: %s lists %d named founders", name, len(founders))
	}
	appendObservation(&result, "wikidata", title, itemURL, data)
	return discovered, result
}

func wikidataMatchesTarget(entity wikidataEntity, target model.Target) bool {
	for _, website := range wikidataValues[string](entity, "P856") {
		if matchingHost(website, target.Domain) {
			return true
		}
		for _, alias := range target.Aliases {
			if matchingHost(website, alias) {
				return true
			}
		}
	}
	return false
}

func wikidataLogo(entities ...wikidataEntity) string {
	for _, entity := range entities {
		for _, logo := range wikidataValues[string](entity, "P154") {
			if logo = strings.TrimSpace(logo); logo != "" && !strings.ContainsAny(logo, "/\\") {
				return "https://commons.wikimedia.org/wiki/Special:FilePath/" + url.PathEscape(logo) + "?width=256"
			}
		}
	}
	return ""
}
