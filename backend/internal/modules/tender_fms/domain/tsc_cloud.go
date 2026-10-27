package domain

// Cloud allowances are intentionally conservative, retained on ambiguous calls,
// and separate from a provider's invoiced usage. All money is integer paise.
const TSCCloudBudgetPaise int64 = 10000
const TSCCloudProposalVersion = "tsc-cloud-cited-answer-only-v3"

type TSCCloudCall struct {
	ID, DraftID, JobID, ActorID, Key, Fingerprint, Provider, Model, State, Error string
	ReservedPaise                                                                int64
	Answer                                                                       TSCAnswer
	Input                                                                        TSCCloudIdentity
}

type TSCCloudIdentity struct {
	DraftVersion, AnswerVersion int64
	Question                    TSCQuestion
	Passages                    []TSCPassage
}

type TSCProviderOption struct {
	Mode           string `json:"mode"`
	Model          string `json:"model"`
	Available      bool   `json:"available"`
	Cloud          bool   `json:"cloud"`
	AllowancePaise int64  `json:"allowance_paise"`
}

type TSCOptions struct {
	Providers     []TSCProviderOption `json:"providers"`
	LimitPaise    int64               `json:"limit_paise"`
	ReservedPaise int64               `json:"reserved_paise"`
}

type TSCResearch struct {
	QuestionID string       `json:"question_id"`
	Passages   []TSCPassage `json:"passages"`
	Proposal   *TSCAnswer   `json:"proposal,omitempty"`
	Warning    string       `json:"warning,omitempty"`
}

func TSCCloudAllowance(mode string) int64 {
	switch mode {
	case "gemini":
		return 150
	case "openai":
		return 60
	default:
		return 0
	}
}
