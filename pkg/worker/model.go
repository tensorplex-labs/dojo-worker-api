package worker

import "time"

type WorkerPartner struct {
	Id              string    `json:"id"`
	CreatedAt       time.Time `json:"createdAt"`
	SubscriptionKey string    `json:"subscriptionKey"`
	Name            string    `json:"name"`
}

type ListWorkerPartnersResponse struct {
	Partners []WorkerPartner `json:"partners"`
}
