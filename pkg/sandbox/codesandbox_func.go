package sandbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"math/rand"
	"time"
	"io"

	"github.com/playwright-community/playwright-go"
	"github.com/rs/zerolog/log"
)

type Response struct {
	Sandbox_id string `json:"sandbox_id"`
	Error      string `json:"error"`
	Url        string `json:"-"`
}

var (
	pwBrowser playwright.Browser
	once      sync.Once
)

func GetBrowser() playwright.Browser {
	once.Do(func() {
		pw, err := playwright.Run()
		if err != nil {
			log.Fatal().Err(err).Msg("could not start playwright")
		}
		firefox, err := pw.Firefox.Launch(
			playwright.BrowserTypeLaunchOptions{
				Headless: playwright.Bool(true),
			},
		)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not launch browser")
		}
		pwBrowser = firefox
	})
	return pwBrowser
}

func activateSandbox(sandboxId string) {
	browser := GetBrowser()
	if browser == nil {
		log.Error().Msg("Error getting browser")
		return
	}
	browserContext, err := browser.NewContext(
		playwright.BrowserNewContextOptions{},
	)
	if err != nil {
		log.Fatal().Msgf("could not start playwright: %v", err)
	}
	page, err := browserContext.NewPage()
	if err != nil {
		log.Fatal().Msgf("could not create page: %v", err)
	}
	page.SetViewportSize(1920, 1080)
	_, err = page.Goto(fmt.Sprintf("https://codesandbox.io/p/redirect-to-project-editor/%s", sandboxId))
	if err != nil {
		log.Fatal().Msgf("could not goto: %v", err)
	}
	time.Sleep(20 * time.Second)
	if err = browserContext.Close(); err != nil {
		log.Fatal().Msgf("could not close browser: %v", err)
	}
}

func getRequest(body map[string]interface{}) (Response, error) {
	var response Response
	url := "https://codesandbox.io/api/v1/sandboxes/define?json=1"
	body["environment"] = "server"
	jsonBody, err := json.Marshal(body)
	if err != nil {
		response.Error = "Error marshalling JSON"
		return response, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		response.Error = "Error creating request"
		return response, err
	}
	req.Header.Set("Content-Type", "application/json")
	// req.Header.Set("_cfuvid", os.Getenv("CODESANDBOX_ID"))
	// req.Header.Set(os.Getenv("CODESANDBOX_KEY"), os.Getenv("CODESANDBOX_KEY_VALUE"))

	client := &http.Client{}

	maxRetries := 3
	retryDelay := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		resp, err := client.Do(req)
		if err != nil {
			response.Error = "Error sending request"
			return response, err
		}
		defer resp.Body.Close()

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Error().Msgf("Failed to read response body: %v", err)
			response.Error = "Failed to read response body"
			return response, fmt.Errorf("failed to read response body: %w", err)
		}
	
		// Log the response payload
		log.Info().Msgf("Response payload (attempt %d): %s", i+1, string(body))
		log.Info().Msgf("Response status code: %d", resp.StatusCode)

		if resp.StatusCode == http.StatusInternalServerError {
			if i < maxRetries-1 {
				jitter := time.Duration(rand.Float64())
				delay := retryDelay + jitter
				log.Warn().Msgf("Request failed with status code 500. Retrying in %v...", delay)
				time.Sleep(delay)
				continue
			} else {
				response.Error = "Internal Server Error"
				return response, fmt.Errorf("server returned 500 status code after %d retries", maxRetries)
			}
		}

		// Request was successful, break out of the retry loop
		if err := json.NewDecoder(bytes.NewReader(body)).Decode(&response); err != nil {
			log.Error().Msgf("Failed to decode JSON response: %v", err)
			response.Error = "Failed to decode JSON response"
			return response, fmt.Errorf("failed to decode JSON response: %w", err)
		}
		break
	}

	return response, nil
}

func reformatFiles(files []interface{}, python bool) map[string]interface{} {
	newFiles := make(map[string]interface{})
	for _, file := range files {
		file := file.(map[string]interface{})
		fileName := file["filename"].(string)
		// handle package.json contents which should be a map instead of json string
		if strings.EqualFold(fileName, "package.json") {
			var packageJson map[string]interface{}
			err := json.Unmarshal([]byte(file["content"].(string)), &packageJson)
			if err != nil {
				log.Error().Msgf("Failed to unmarshal package.json content: %v", err)
				continue
			}
			newFiles[fileName] = map[string]interface{}{
				"content": packageJson,
			}
		} else {
			newFiles[fileName] = map[string]interface{}{
				"content": file["content"],
				// "language": file["language"],
			}
		}
	}

	if python {
		newFiles["main.py"].(map[string]interface{})["isBinary"] = false
	}
	return newFiles
}

func GetCodesandbox(body map[string]interface{}) (Response, error) {
	var response Response
	files, ok := body["files"].([]interface{})
	if !ok {
		log.Error().Msg("Error getting files")
		response.Error = "Error getting files"
		return response, errors.New("object has no files key")
	}
	standard_language := false
	python := false
	for _, file := range files {
		file, ok := file.(map[string]interface{})
		if !ok {
			log.Error().Msg("Error getting file")
			response.Error = "Error getting file"
			return response, errors.New("file object is not a map")
		}
		language, ok := file["language"]
		if !ok {
			log.Error().Msg("Error getting language")
			response.Error = "Error getting language"
			return response, errors.New("files object has no language key")
		}

		if strings.EqualFold(language.(string), "javascript") || strings.EqualFold(language.(string), "html"){
			standard_language = true
		} else if strings.ToLower(language.(string)) == "python" {
			python = true
		}
	}
	body["files"] = reformatFiles(body["files"].([]interface{}), python)

	response, err := getRequest(body)
	if err != nil {
		response.Error = "Error getting request"
		log.Error().Msg("Error getting request")
		return response, err
	}
	log.Info().Msgf("Sandbox ID: %s", response.Sandbox_id)
	log.Info().Msgf("Sandbox error: %s", response.Error)

	go activateSandbox(response.Sandbox_id)

	if standard_language {
		response.Url = "https://" + response.Sandbox_id + ".csb.app/"
	} else if python {
		response.Url = "https://" + response.Sandbox_id + "-8888.csb.app/"
	} else {
		log.Error().Msg("Invalid language")
		response.Error = "Invalid language"
		return response, errors.New("invalid language")
	}
	return response, nil
}
