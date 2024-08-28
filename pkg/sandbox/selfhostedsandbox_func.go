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

	jsString, err := getFileContent(files, "index.js")
	if err != nil {
		response.Error = "Error getting index.js content"
		return response, err
	}

	cssContent, htmlWithoutCSS, err := extractCSS(htmlString)
	if err != nil {
		response.Error = "Error extracting CSS"
		return response, err
	}

	response.CombinedHTML = injectContent(htmlWithoutCSS, cssContent, jsString)

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

func extractCSS(htmlString string) (string, string, error) {
	cssStart := strings.Index(htmlString, "<style>")
	cssEnd := strings.Index(htmlString, "</style>")
	if cssStart == -1 || cssEnd == -1 {
		return "", "", fmt.Errorf("CSS not found in HTML")
	}

	cssContent := htmlString[cssStart+7 : cssEnd]
	htmlWithoutCSS := htmlString[:cssStart] + htmlString[cssEnd+8:]

	return cssContent, htmlWithoutCSS, nil
}

func injectContent(html, css, js string) string {
	html = strings.Replace(html, "</head>", fmt.Sprintf("<style>%s</style></head>", css), 1)
	html = strings.Replace(html, "</body>", fmt.Sprintf("<script>%s</script></body>", js), 1)
	return html
}
