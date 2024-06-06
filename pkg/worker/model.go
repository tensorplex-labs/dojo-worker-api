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
	MinerSubscriptionKey    string `json:"minerSubscriptionKey" binding:"required"`
	NewMinerSubscriptionKey string `json:"newMinerSubscriptionKey" binding:"required"`
	Name                    string `json:"name" binding:"required"`
}

type UpdateWorkerPartnerResponse struct {
	WorkerPartner WorkerPartner `json:"workerPartner"`
}

type DisableMinerRequest struct {
	MinerSubscriptionKey string `json:"minerSubscriptionKey" binding:"required"`
	ToDisable            bool   `json:"toDisable" binding:"required"`
}

type DisableSuccessResponse struct {
	Message string `json:"message"`
}

// type DisableWorkerRequest struct {
// 	WorkerId  string `json:"workerId" binding:"required"`
// 	ToDisable bool   `json:"toDisable" binding:"required"`
// }

type WorkerLoginRequest struct {
	WalletAddress string `json:"walletAddress" binding:"required"`
	ChainId       string `json:"chainId" binding:"required"`
	Message       string `json:"message" binding:"required"`
	Signature     string `json:"signature" binding:"required"`
	Timestamp     string `json:"timestamp" binding:"required"`
}

type WorkerLoginSuccessResponse struct {
	Token any `json:"token"`
}

type WorkerPartnerCreateRequest struct {
	Name                 string `json:"name" binding:"required"`
	MinerSubscriptionKey string `json:"minerSubscriptionKey" binding:"required"`
}

type GenerateNonceResponse struct {
	Nonce string `json:"nonce"`
}
