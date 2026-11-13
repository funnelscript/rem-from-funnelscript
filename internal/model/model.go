package model

const Version = "0.1.0"

type Target struct {
	Input   string   `json:"input"`
	Kind    string   `json:"kind"`
	URL     string   `json:"url,omitempty"`
	Domain  string   `json:"domain,omitempty"`
	Brand   string   `json:"brand,omitempty"`
	AppleID string   `json:"appleId,omitempty"`
	PlayID  string   `json:"playId,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
}

type Evidence struct {
	ID         string `json:"id"`
	Source     string `json:"source"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	URL        string `json:"url,omitempty"`
	ObservedAt string `json:"observedAt"`
	Data       any    `json:"data"`
}

type Unknown struct {
	Channel     string `json:"channel"`
	Reason      string `json:"reason"`
	ManualCheck string `json:"manualCheck,omitempty"`
}

type Content struct {
	ID       string             `json:"id"`
	Platform string             `json:"platform"`
	Kind     string             `json:"kind"`
	URL      string             `json:"url"`
	Title    string             `json:"title,omitempty"`
	Text     string             `json:"text,omitempty"`
	Thumb    string             `json:"thumb,omitempty"`
	Video    string             `json:"video,omitempty"`
	Author   string             `json:"author,omitempty"`
	Date     string             `json:"date,omitempty"`
	Status   string             `json:"status,omitempty"`
	Evidence string             `json:"evidence,omitempty"`
	Metrics  map[string]float64 `json:"metrics,omitempty"`
	Tags     []string           `json:"tags,omitempty"`
}

type Discovered struct {
	Socials     map[string]string `json:"socials"`
	AppStoreIDs []string          `json:"appStoreIds,omitempty"`
	PlayIDs     []string          `json:"playIds,omitempty"`
	Logos       []Logo            `json:"logos,omitempty"`
	Founders    []Founder         `json:"founderHints,omitempty"`
	BrandName   string            `json:"brandName,omitempty"`
}

type Logo struct {
	URL    string `json:"url"`
	Source string `json:"source"`
}

type Founder struct {
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
	URL  string `json:"url,omitempty"`
}

type Result struct {
	Evidence []Evidence
	Unknowns []Unknown
	Content  []Content
}

type SourceRun struct {
	Source string `json:"source"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	MS     int64  `json:"ms"`
}

type ChannelScore struct {
	Channel     string   `json:"channel"`
	Status      string   `json:"status"`
	Strength    int      `json:"strength"`
	Signal      string   `json:"signal"`
	EvidenceIDs []string `json:"evidence"`
}

type Identity struct {
	Logo     string    `json:"logo,omitempty"`
	Logos    []Logo    `json:"logos"`
	Founders []Founder `json:"founders"`
}

type Metadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Harness struct {
	Version string `json:"version"`
	SHA     string `json:"sha"`
}

type Credits struct {
	ScrapeCreators *int `json:"scrapecreators"`
}

type Bundle struct {
	ID            string         `json:"id,omitempty"`
	Tool          Metadata       `json:"tool"`
	Harness       Harness        `json:"harness"`
	Target        Target         `json:"target"`
	GeneratedAt   string         `json:"generatedAt"`
	Brand         string         `json:"brand"`
	Discovered    Discovered     `json:"discovered"`
	Scorecard     []ChannelScore `json:"scorecard"`
	Evidence      []Evidence     `json:"evidence"`
	Unknowns      []Unknown      `json:"unknowns"`
	Sources       []SourceRun    `json:"sources"`
	Credits       Credits        `json:"credits"`
	Content       []Content      `json:"content"`
	Identity      Identity       `json:"identity"`
	ReportWarning string         `json:"reportWarning,omitempty"`
}

type Summary struct {
	ID            string `json:"id"`
	Brand         string `json:"brand"`
	Domain        string `json:"domain"`
	GeneratedAt   string `json:"generatedAt"`
	EvidenceCount int    `json:"evidenceCount"`
	UnknownCount  int    `json:"unknownCount"`
}
