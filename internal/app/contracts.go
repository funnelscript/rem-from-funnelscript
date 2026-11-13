package app

type Options struct {
	Only       []string
	Handles    map[string]string
	MaxCredits int
	BudgetSet  bool
	Engine     string
	Model      string
	OnStage    func(stage, state string)
}
