package api
import (
	"dojo-api/utils"
	"errors"
	"fmt"
)


func ValidateTaskRequest(taskData utils.TaskRequest) error {
	if taskData.Title == "" {
		return errors.New("title is required")
	}

	if taskData.Body == "" {
		return errors.New("body is required")
	}

	if taskData.ExpireAt == "" {
		return errors.New("expireAt is required")
	}

	for _, taskInterface := range taskData.TaskData {
		err := validateTaskData(taskInterface)
		if err != nil {
			return err
		}
	}

	if taskData.MaxResults == 0 {
		return errors.New("maxResults is required")
	}

	if taskData.TotalRewards == 0 {
		return errors.New("totalRewards is required")
	}

	return nil
}

func ProcessTaskRequest(taskData *utils.TaskRequest) error {
	for i, taskInterface := range taskData.TaskData {
		if taskInterface.Task == "CODE_GENERATION" {
			ProcessCodeCompletion(&taskData.TaskData[i])
		}
	}
	return nil
}

func validateTaskData(taskData utils.TaskData) error {
	if taskData.Prompt == "" && len(taskData.Dialogue) == 0{
		return errors.New("prompt is required")
	}

	if taskData.Task == "" {
		return errors.New("task is required")
	}
	task := taskData.Task
	if task == "CODE_GENERATION" || task == "TEXT_TO_IMAGE" {
		for _, taskresponse := range taskData.Responses {
			var ok bool
			if task == "CODE_GENERATION" {
				_, ok = taskresponse.Completion.(map[string]interface{})
			} else if task == "TEXT_TO_IMAGE" {
				_, ok = taskresponse.Completion.(string)
			}
			if !ok {
				return fmt.Errorf("invalid completion format: %v", taskresponse.Completion)
			}
		}

		if len(taskData.Dialogue) != 0 {
			return errors.New("dialogue should be empty for code generation and text to image tasks")
		}
	// TODO: change to dialogue when schema is updated
	} else if task == "CONVERSATION" {
		if len(taskData.Responses) != 0 {
			return errors.New("responses should be empty for dialogue task")
		}

		if len(taskData.Dialogue) == 0 {
			return errors.New("dialogue is required for dialogue task")
		}
	}else {
		return errors.New("invalid task")
	}

	if len(taskData.Criteria) == 0 {
		return errors.New("criteria is required")
	}

	for _, criteria := range taskData.Criteria {
		if criteria.Type == "" {
			return errors.New("type is required for criteria")
		}

		if criteria.Type == "multi-select" || criteria.Type == "ranking" {
			if len(criteria.Options) == 0 {
				return errors.New("options is required for multiple choice criteria")
			}
		} else if criteria.Type == "score" {
			if criteria.Min == 0 && criteria.Max == 0 {
				return errors.New("min or max is required for numeric criteria")
			}
		}else {
			return errors.New("invalid criteria type")
		}
	}

	return nil
}

func ProcessCodeCompletion(taskData *utils.TaskData) error{
	responses := taskData.Responses
	logger := utils.GetLogger()
	for i, response := range responses {
		completion := response.Completion
		completionMap, ok := completion.(map[string]interface{})
		if !ok {
			logger.Error().Msg("You sure this is code generation?")
			return errors.New("invalid completion format")
		}
		if completionMap["language"] == "python" {
			pythonResponse, err := utils.GetPythonUrl(completionMap["code"].(string))
			if err != nil {
				logger.Error().Msg(fmt.Sprintf("Error getting python response: %v", err))
				return err
			}
			if pythonResponse.Success {
				completionMap["sandbox_url"] = pythonResponse.Body.Fileurl
			}else {
				logger.Error().Msg(fmt.Sprintf("Error getting python response: %v", pythonResponse.Error))
				return errors.New("error getting python response")
			}	
		}else if _, ok := completionMap["files"]; ok{
			javascriptResponse, err := utils.GetCodesandbox(completionMap)
			if err != nil {
				logger.Error().Msg(fmt.Sprintf("Error getting javascript response: %v", err))
				return err
			}
			if javascriptResponse.Sandbox_id != "" {
				completionMap["sandbox_url"] = "https://" + javascriptResponse.Sandbox_id + ".csb.app/"
			}else {
				fmt.Println(javascriptResponse)
				logger.Error().Msg("Error getting javascript response")
				return errors.New("error getting javascript response")
			}
		}else {
			logger.Error().Msg("Invalid completion format")
			return errors.New("invalid completion format")
		}
		taskData.Responses[i].Completion = completionMap
	}
	return nil
}