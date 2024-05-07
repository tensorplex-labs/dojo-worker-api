package api

import (
	"dojo-api/pkg/cache"
	"sync"
	"time"
)

var (
	cacheInstance *cache.Cache
	once          sync.Once
)

// Define a common response structure
type ApiResponse struct {
	Success bool        `json:"success"`
	Body    interface{} `json:"body"`
	Error   interface{} `json:"error"`
}

func defaultErrorResponse(errorMsg interface{}) ApiResponse {
	return ApiResponse{Success: false, Body: nil, Error: errorMsg}
}

func defaultSuccessResponse(body interface{}) ApiResponse {
	return ApiResponse{Success: true, Body: body, Error: nil}
}

func GetCacheInstance() *cache.Cache {
	once.Do(func() {
		cacheInstance = cache.NewCache(10000, 10*time.Minute)
	})
	return cacheInstance
}
