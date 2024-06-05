package worker

import (
	"time"
)

type WorkerPartner struct {
	Id              string    `json:"id"`
	CreatedAt       time.Time `json:"createdAt"`
	SubscriptionKey string    `json:"subscriptionKey"`
	Name            string    `json:"name"`
}

type ListWorkerPartnersResponse struct {
	Partners []WorkerPartner `json:"partners"`
}

// no usage but for go swagger
type UpdateWorkerPartnerRequest struct {
	MinerSubscriptionKey    string `json:"minerSubscriptionKey"`
	NewMinerSubscriptionKey string `json:"newMinerSubscriptionKey"`
	Name                    string `json:"name"`
}

type UpdateWorkerPartnerResponse struct {
	WorkerPartner WorkerPartner `json:"workerPartner"`
}

// no usage but for go swagger
type DisableMinerRequest struct {
	MinerSubscriptionKey string `json:"minerSubscriptionKey"`
	ToDisable            bool   `json:"toDisable"`
}

type DisableSuccessResponse struct {
	Message string `json:"message"`
}

// no usage but for go swagger
type DisableWorkerRequest struct {
	WorkerId  string `json:"workerId"`
	ToDisable bool   `json:"toDisable"`
}

// no usage but for go swagger
type WorkerLoginRequest struct {
	WalletAddress string `json:"walletAddress"`
	ChainId       string `json:"chainId"`
	Message       string `json:"message"`
	Signature     string `json:"signature"`
	Timestamp     string `json:"timestamp"`
}

type WorkerLoginSuccessResponse struct {
	Token any `json:"token"`
}

// no usage but for go swagger
type WorkerPartnerCreateRequest struct {
	Name                 string `json:"name"`
	MinerSubscriptionKey string `json:"minerSubscriptionKey"`
}

type GenerateNonceResponse struct {
	Nonce string `json:"nonce"`
}
