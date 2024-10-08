package miner

type MinerApplicationRequest struct {
	Hotkey           string `json:"hotkey"`
	Email            string `json:"email"`
	OrganisationName string `json:"organisationName,omitempty"`
}

type MinerInfoResponse struct {
	MinerId         string `json:"minerId"`
	SubscriptionKey string `json:"subscriptionKey"`
}

type MinerApiKeysResponse struct {
	ApiKeys []string `json:"apiKeys"`
}

type MinerApiKeyDisableRequest struct {
	ApiKey string `json:"apiKey"`
}

type MinerSubscriptionKeysResponse struct {
	SubscriptionKeys []string `json:"subscriptionKeys"`
}

type MinerSubscriptionDisableRequest struct {
	SubscriptionKey string `json:"subscriptionKey"`
}
