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
