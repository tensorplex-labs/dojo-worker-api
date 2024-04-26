package utils

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	// "fmt"
)

type JavascriptResponse struct {
	Sandbox_id string `json:"sandbox_id"`
	Error string `json:"error"`
}

type PythonResponseBody struct {
	Fileurl string `json:"url_to_file"`
}

type PythonResponse struct {
	Success bool `json:"success"`
	Body PythonResponseBody `json:"body"`
	Error string `json:"error"`
}

func GetPythonUrl(code string) (PythonResponse, error) {
	var response PythonResponse
	var request = map[string]interface{}{
		"code" : code,
	}
	logger := GetLogger()
	port := os.Getenv("SERVER_PORT")
    if port == "" {
        port = "8080" // Default port if not specified
    }
	url := "http://localhost:" + "5003" + "/api/codegen-python/"

	jsonBody, err := json.Marshal(request)
	if err != nil {
		response.Error = "Error marshaling body"
		logger.Error().Msgf("Error marshaling body: %v", err)
		return response, err
	}
	req,err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		response.Error = "Error creating request"
		logger.Error().Msgf("Error creating request: %v", err)
		return response, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
		response.Error = "Error sending request"
		logger.Error().Msgf("Error sending request: %v", err)
		return response, err
    }
    defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&response)
	return response, nil
}

func GetCodesandbox(body map[string]interface{}) (JavascriptResponse, error) {
	var response JavascriptResponse
	logger := GetLogger()
	url := "https://codesandbox.io/api/v1/sandboxes/define?json=1"
	jsonBody, err := json.Marshal(body)
	if err != nil {
		response.Error = "Error marshaling body"
		logger.Error().Msgf("Error marshaling body: %v", err)
		return response, err
	}
	req,err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		response.Error = "Error creating request"
		logger.Error().Msgf("Error creating request: %v", err)
		return response, err
	}
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
		response.Error = "Error sending request"
		logger.Error().Msgf("Error sending request: %v", err)
		return response, err
    }
    defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(&response)
	return response, nil
}






