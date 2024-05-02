package api

import (
	"sync"

	"github.com/bluele/gcache"
)

var (
	cacheInstance gcache.Cache
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

func GetCacheInstance() gcache.Cache {
	once.Do(func() {
		cacheInstance = gcache.New(100).ARC().Build()
	})
	return cacheInstance
}
