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

type GenerateCookieAuthRequest struct {
	Hotkey    string `json:"hotkey"`
	Signature string `json:"signature"`
	Message   string `json:"message"`
}

// HashKey and BlockKey are used to encrypt and decrypt the cookie data, MUST NEVER BE EXPOSED
// HashKey and BlockKey are used to encrypt and decrypt the cookie data, MUST NEVER BE EXPOSED
// HashKey and BlockKey are used to encrypt and decrypt the cookie data, MUST NEVER BE EXPOSED
// HashKey and BlockKey are used to encrypt and decrypt the cookie data, MUST NEVER BE EXPOSED
type SecureCookieSession struct {
	HashKey  []byte `json:"hashKey"`
	BlockKey []byte `json:"blockKey"`
	// just to check if data was tampered with
	CookieData
}

type CookieData struct {
	SessionId string `json:"sessionId"`
	Hotkey    string `json:"hotkey"`
}
