package app

type Options struct {
	Only       []string
	Handles    map[string]string
	MaxCredits int
	BudgetSet  bool
	Engine     string
	Model      string
	Stealth    bool
	Keyless    bool
	Window     string
	OnStage    func(stage, state string)
}
