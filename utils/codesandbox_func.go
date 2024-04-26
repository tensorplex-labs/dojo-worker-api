package utils

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type Response struct {
	Sandbox_id string `json:"sandbox_id"`
	Error string `json:"error"`
}

func GetCodesandbox(body map[string]interface{}) Response {
	url := "https://codesandbox.io/api/v1/sandboxes/define?json=1"
	jsonBody, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	req,err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

	var response Response
	json.NewDecoder(resp.Body).Decode(&response)
	return response
}






