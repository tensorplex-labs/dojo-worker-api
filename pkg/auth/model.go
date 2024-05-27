package auth

type MinerLoginRequest struct {
	Hotkey       string `json:"hotkey"`
	Signature    string `json:"signature"`
	Message      string `json:"message"`
	Email        string `json:"email"`
	Organisation string `json:"organisation,omitempty"`
}

type MinerLoginResponse struct {
	ApiKey          string `json:"apiKey"`
	SubscriptionKey string `json:"subscriptionKey"`
}
