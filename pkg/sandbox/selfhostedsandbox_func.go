package sandbox

import (
	"fmt"
	"strings"
)

type FileContent struct {
	Content interface{} `json:"content"`
}

type Files struct {
	Files map[string]FileContent `json:"files"`
}

type CombinedHTMLResponse struct {
	Error        string
	CombinedHTML string
}

func CombineFiles(filesMap map[string]interface{}) (CombinedHTMLResponse, error) {
	var response CombinedHTMLResponse
	files, err := extractFiles(filesMap)
	if err != nil {
		response.Error = "Error extracting files"
		return response, err
	}

	htmlString, err := getFileContent(files, "index.html")
	if err != nil {
		response.Error = "Error getting index.html content"
		return response, err
	}
	if htmlString == "" {
		response.Error = "index.html content is empty"
		return response, fmt.Errorf("%s", response.Error)
	}

	jsString, err := getFileContent(files, "index.js")
	if err != nil {
		jsString = ""
	}
	cssString := getCSSContent(files)
	response.CombinedHTML = injectContent(htmlString, cssString, jsString)
	return response, nil
}

func extractFiles(filesMap map[string]interface{}) (Files, error) {
	filesArray, ok := filesMap["files"].([]interface{})
	if !ok {
		return Files{}, fmt.Errorf("files array not found in input")
	}

	files := Files{
		Files: make(map[string]FileContent),
	}

	for _, fileInterface := range filesArray {
		file, ok := fileInterface.(map[string]interface{})
		if !ok {
			return Files{}, fmt.Errorf("invalid file structure in array")
		}
		filename, ok := file["filename"].(string)
		if !ok {
			return Files{}, fmt.Errorf("filename not found or not a string")
		}
		content, ok := file["content"].(string)
		if !ok {
			return Files{}, fmt.Errorf("content not found or not a string for file: %s", filename)
		}
		files.Files[filename] = FileContent{
			Content: content,
		}
	}

	return files, nil
}

func getFileContent(files Files, filename string) (string, error) {
	content, ok := files.Files[filename]
	if !ok {
		return "", fmt.Errorf("%s not found in input", filename)
	}

	contentString, ok := content.Content.(string)
	if !ok {
		return "", fmt.Errorf("%s content is not a string", filename)
	}

	return contentString, nil
}

func getCSSContent(files Files) string {
	for filename, content := range files.Files {
		if strings.HasSuffix(filename, ".css") {
			if cssString, ok := content.Content.(string); ok {
				return cssString
			}
		}
	}
	return ""
}

func injectContent(html, css, js string) string {
	if css != "" {
		html = strings.Replace(html, "</head>", fmt.Sprintf("<style>%s</style></head>", css), 1)
	}
	if js != "" {
		html = strings.Replace(html, "</body>", fmt.Sprintf("<script>%s</script></body>", js), 1)
	}
	return html
}
