package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"bytes"
	"encoding/json"
	// "github.com/bitrise-io/go-utils/log"
	"io/ioutil"
	"net/http"
	"strings"
)
type JiraRequestData struct {
	JiraUsername     string
	JiraPassword     string
	JiraInstanceURL  string
	IssueIDOrKeyList []string
	TransitionId     string
}

func main() {
	configs := buildConfigFromEnv()
	if err := configs.validateJiraUsername(); err != nil {
		//log.Errorf("Issue with input: %s", err)
		os.Exit(1)
	}

	if err := configs.validateJiraPassword(); err != nil {
		//log.Errorf("Issue with input: %s", err)
		os.Exit(2)
	}

	if err := configs.validateJiraInstanceURL(); err != nil {
		//log.Errorf("Issue with input: %s", err)
		os.Exit(3)
	}

	if err := configs.validateIssueIDOrKeyList(); err != nil {
		//log.Errorf("Issue with input: %s", err)
		os.Exit(4)
	}

	if err := configs.validateTransitionId(); err != nil {
		//log.Errorf("Issue with input: %s", err)
		os.Exit(5)
	}

	body, err := buildRequestBody(configs)
	if err != nil {
		os.Exit(6);
	}

	for ind, issueIDOrKey := range configs.IssueIDOrKeyList {
		// if err := triggerIssueTransition(configs, idOrKey, body); err != nil {
		// 	errNumber := 20
		// 	os.Exit(errNumber)
		// }

		request, err := buildRequest(configs, issueIDOrKey, body)
		if err != nil {
			os.Exit(11)
			return
		}

		client := http.Client{}
		response, err := client.Do(request)

		if err != nil {
			os.Exit(12)
			return
		}

		defer func() {
			err := response.Body.Close()
			if err != nil {
				//log.Warnf("Failed to close response body, error: %s", err)
				os.Exit(13)
			}
		}()

		if response.StatusCode != http.StatusNoContent {
			//log.Warnf("JIRA API response status: %s", response.Status)
			_, readErr := ioutil.ReadAll(response.Body)
			if readErr != nil {
				os.Exit(14)
			}
			if response.Header.Get("X-Seraph-LoginReason") == "AUTHENTICATION_DENIED" {
				//log.Warnf("CAPTCHA triggered")
			} else {
				//log.Warnf("JIRA API response: %s", contents)
			}
		}
	}
}

func buildConfigFromEnv() JiraRequestData {
	configs := JiraRequestData{
		JiraUsername:     os.Getenv("jira_username"),
		JiraPassword:     os.Getenv("jira_password"),
		JiraInstanceURL:  os.Getenv("jira_instance_url"),
		IssueIDOrKeyList: strings.Split(os.Getenv("issue_id_or_key_list"), "|"),
		TransitionId:         os.Getenv("transition_id"),
	}
	for i, idOrKey := range configs.IssueIDOrKeyList {
		configs.IssueIDOrKeyList[i] = strings.TrimSpace(idOrKey)
	}
	return configs
}

func (configs JiraRequestData) validateJiraUsername() error {
	if configs.JiraUsername == "" {
		return errors.New("no Jira Username specified")
	}
	return nil
}

func (configs JiraRequestData) validateJiraPassword() error {
	if configs.JiraPassword == "" {
		return errors.New("no Jira Password specified")
	}
	return nil
}

func (configs JiraRequestData) validateJiraInstanceURL() error {
	_, err := url.ParseRequestURI(configs.JiraInstanceURL)
	if err != nil {
		return fmt.Errorf("invalid JiraInstanceURL, error %s", err)
	}
	return nil
}

func (configs JiraRequestData) validateIssueIDOrKeyList() error {
	if len(configs.IssueIDOrKeyList) == 0 {
		return errors.New("no issues specified")
	}
	return nil
}

func (configs JiraRequestData) validateTransitionId() error {
	if configs.TransitionId == "" {
		return errors.New("No transition ID")
	}
	return nil
}

// -------------- request related methods -----------------

func buildRequestBody(configs JiraRequestData) ([]byte, error) {
	payload := map[string]interface{}{
		"transition": map[string]interface{}{
			"id": configs.TransitionId,
		},
	}
	return json.Marshal(payload)
}

func buildRequest(configs JiraRequestData, issueIDOrKey string, body []byte) (*http.Request, error) {
	requestURL := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", configs.JiraInstanceURL, issueIDOrKey)
	request, err := http.NewRequest("POST", requestURL, bytes.NewBuffer(body))
	if err != nil {
		return request, err
	}

	request.SetBasicAuth(configs.JiraUsername, configs.JiraPassword)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	return request, nil
}

func triggerIssueTransition(configs JiraRequestData, issueIDOrKey string, body []byte) error {
	//log.Infof("Triggering for issue %s", issueIDOrKey)

	request, err := buildRequest(configs, issueIDOrKey, body)
	if err != nil {
		return err
	}

	client := http.Client{}
	response, err := client.Do(request)

	if err != nil {
		return err
	}

	defer func() {
		err := response.Body.Close()
		if err != nil {
			//log.Warnf("Failed to close response body, error: %s", err)
		}
	}()

	if response.StatusCode != http.StatusNoContent {
		//log.Warnf("JIRA API response status: %s", response.Status)
		_, readErr := ioutil.ReadAll(response.Body)
		if readErr != nil {
			return errors.New("could not read JIRA API response")
		}
		if response.Header.Get("X-Seraph-LoginReason") == "AUTHENTICATION_DENIED" {
			//log.Warnf("CAPTCHA triggered")
		} else {
			//log.Warnf("JIRA API response: %s", contents)
		}
		return errors.New("JIRA API request failed")
	}

	//log.Infof("Issue %s updated successfully", issueIDOrKey)
	return nil
}

// Perform a series of requests with pre-validated configs
func performRequests(configs JiraRequestData) error {
	body, err := buildRequestBody(configs)
	if err != nil {
		return err
	}

	for _, idOrKey := range configs.IssueIDOrKeyList {
		if err := triggerIssueTransition(configs, idOrKey, body); err != nil {
			return err
		}
	}

	return nil
}