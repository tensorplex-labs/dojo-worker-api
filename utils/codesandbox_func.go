package utils

import (
	"bytes"
	"encoding/json"
	"net/http"
	"errors"
	"github.com/rs/zerolog/log"
	"fmt"
	"reflect"
	// "os"
	"github.com/joho/godotenv"
)

type Response struct {
	Sandbox_id string `json:"sandbox_id"`
	Error string `json:"error"`
	Url string `json:"-"`
}

func getRequest(body map[string]interface{}, response *Response) error {
	url := "https://codesandbox.io/api/v1/sandboxes/define?json=1"
	body["environment"] = "server"
	jsonBody, err := json.Marshal(body); if err != nil {
		return err
	}

	err = godotenv.Load(); if err != nil {
        log.Error().Msg("Error loading .env file")
    }

	req,err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	// req.Header.Set("_cfuvid", os.Getenv("CODESANDBOX_ID"))
	// req.Header.Set(os.Getenv("CODESANDBOX_KEY"), os.Getenv("CODESANDBOX_KEY_VALUE"))
	
	client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
		return err
    }
    defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(response)
	return nil
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
	var r = reflect.TypeOf(body["files"])
	fmt.Printf("Other:%v\n", r) 
	var response Response
	files, ok := body["files"].([]interface{}); if !ok {
		response.Error = "Error getting files"
		log.Error().Msg("Error getting files")
		return response, errors.New("object has no files key")
	}
	javascript := false
	python := false
	for _, file := range files {
		file, ok := file.(map[string]interface{}); if !ok {
			response.Error = "Error getting file"
			log.Error().Msg("Error getting file")
			return response, errors.New("file object is not a map")
		}
		language, ok := file["language"]; if !ok {
			response.Error = "Error getting language"
			log.Error().Msg("Error getting language")
			return response, errors.New("files object has no language key")
		}

		if language == "javascript" {
			javascript = true
		} else if language == "python" {
			python = true
		}
	}
	body["files"] = reformatFiles(body["files"].([]interface{}), python)

	err := getRequest(body, &response); if err != nil {
		response.Error = "Error getting request"
		log.Error().Msg("Error getting request")
		return response, err
	}

	if javascript {
		response.Url = "https://" + response.Sandbox_id + ".csb.app/"
	}else if python {
		response.Url = "https://" + response.Sandbox_id + "-8050.csb.app/"
	}else {
		response.Error = "Invalid language"
		log.Error().Msg("Invalid language")
		return response, errors.New("invalid language")
	}
	return response, nil
}






