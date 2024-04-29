package sandbox

import (
	"bytes"
	"encoding/json"
	"net/http"
	"errors"
	"github.com/rs/zerolog/log"
	"fmt"
)

type Response struct {
	Sandbox_id string `json:"sandbox_id"`
	Error string `json:"error"`
	Url string `json:"-"`
}

func defaultErrorResponse(errorMsg string) Response {
    return Response{Sandbox_id: "", Error: errorMsg, Url: ""}
}

func getRequest(body map[string]interface{}) (Response, error) {
	var response Response
	url := "https://codesandbox.io/api/v1/sandboxes/define?json=1"
	body["environment"] = "server"
	jsonBody, err := json.Marshal(body); if err != nil {
		return defaultErrorResponse(err.Error()), err
	}

	req,err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return defaultErrorResponse(err.Error()), err
	}
	req.Header.Set("Content-Type", "application/json")
	// req.Header.Set("_cfuvid", os.Getenv("CODESANDBOX_ID"))
	// req.Header.Set(os.Getenv("CODESANDBOX_KEY"), os.Getenv("CODESANDBOX_KEY_VALUE"))
	
	client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
		return defaultErrorResponse(err.Error()), err
    }
    defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Error().Msgf("Failed to decode JSON response: %v", err)
		return defaultErrorResponse("Failed to decode JSON response"), fmt.Errorf("failed to decode JSON response: %w", err)
	}	
	return response, nil
}

func reformatFiles(files []interface{}, python bool) map[string]interface{} {
	newFiles := make(map[string]interface{})
	for _, file := range files {
		file := file.(map[string]interface{})
		newFiles[file["filename"].(string)] = map[string]interface{}{
			"content": file["content"],
			"language": file["language"],
		}
	}

	if python {
		newFiles["main.py"].(map[string]interface{})["isBinary"] = false
	}
	return newFiles
}

func GetCodesandbox(body map[string]interface{}) (Response, error) {
	files, ok := body["files"].([]interface{}); if !ok {
		log.Error().Msg("Error getting files")
		return defaultErrorResponse("Error getting files"), errors.New("object has no files key")
	}
	javascript := false
	python := false
	for _, file := range files {
		file, ok := file.(map[string]interface{}); if !ok {
			log.Error().Msg("Error getting file")
			return defaultErrorResponse("Error getting file"), errors.New("file object is not a map")
		}
		language, ok := file["language"]; if !ok {
			log.Error().Msg("Error getting language")
			return defaultErrorResponse("Error getting language"), errors.New("files object has no language key")
		}

		if language == "javascript" {
			javascript = true
		} else if language == "python" {
			python = true
		}
	}
	body["files"] = reformatFiles(body["files"].([]interface{}), python)

	response, err := getRequest(body); 
	if err != nil {
		response.Error = "Error getting request"
		log.Error().Msg("Error getting request")
		return response, err
	}

	if javascript {
		response.Url = "https://" + response.Sandbox_id + ".csb.app/"
	}else if python {
		response.Url = "https://" + response.Sandbox_id + "-8050.csb.app/"
	}else {
		log.Error().Msg("Invalid language")
		return defaultErrorResponse("Invalid language"), errors.New("invalid language")
	}
	return response, nil
}






