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

	htmlString, err := getFileContentBySuffix(files, ".html")
	if err != nil {
		response.Error = "Error getting HTML content"
		return response, err
	}
	if htmlString == "" {
		response.Error = "HTML content is empty"
		return response, fmt.Errorf("%s", response.Error)
	}

	jsString, _ := getFileContentBySuffix(files, ".js")
	cssString, _ := getFileContentBySuffix(files, ".css")

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

func getFileContentBySuffix(files Files, suffix string) (string, error) {
	for filename, content := range files.Files {
		if strings.HasSuffix(filename, suffix) {
			if contentString, ok := content.Content.(string); ok {
				return contentString, nil
			}
			return "", fmt.Errorf("content for %s is not a string", filename)
		}
	}
	return "", fmt.Errorf("no file with suffix %s found", suffix)
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
