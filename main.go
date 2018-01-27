package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"bytes"
	"encoding/json"
	"github.com/bitrise-io/go-utils/log"
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
	configs.dump()
	if err := configs.validate(); err != nil {
		log.Errorf("Issue with input: %s", err)
		os.Exit(1)
	}

	if err := performRequests(configs); err != nil {
		log.Errorf("Could not update issue, error: %s", err)
		os.Exit(2)
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

func (configs JiraRequestData) dump() {
	fmt.Println()
	log.Infof("Configs:")
	log.Printf(" - JiraUsername: %s", configs.JiraUsername)
	log.Printf(" - JiraPassword (hidden): %s", strings.Repeat("*", 5))
	log.Printf(" - JiraInstanceURL: %s", configs.JiraInstanceURL)
	log.Printf(" - IssueIdOrKeyList: %v", configs.IssueIDOrKeyList)
	log.Printf(" - TransitionId: %s", configs.TransitionId)
}

func (configs JiraRequestData) validate() error {
	if configs.JiraUsername == "" {
		return errors.New("no Jira Username specified")
	}
	if configs.JiraPassword == "" {
		return errors.New("no Jira Password specified")
	}
	_, err := url.ParseRequestURI(configs.JiraInstanceURL)
	if err != nil {
		return fmt.Errorf("invalid Jira instance URL, error %s", err)
	}
	if len(configs.IssueIDOrKeyList) == 0 {
		return errors.New("no Jira issue IDs nor keys specified")
	}
	for i, idOrKey := range configs.IssueIDOrKeyList {
		if idOrKey == "" {
			return fmt.Errorf("empty Jira issue ID nor key specified at index %d", i)
		}
	}
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
	log.Infof("Triggering for issue %s", issueIDOrKey)

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
			log.Warnf("Failed to close response body, error: %s", err)
		}
	}()

	if response.StatusCode != http.StatusNoContent {
		log.Warnf("JIRA API response status: %s", response.Status)
		contents, readErr := ioutil.ReadAll(response.Body)
		if readErr != nil {
			return errors.New("could not read JIRA API response")
		}
		if response.Header.Get("X-Seraph-LoginReason") == "AUTHENTICATION_DENIED" {
			log.Warnf("CAPTCHA triggered")
		} else {
			log.Warnf("JIRA API response: %s", contents)
		}
		return errors.New("JIRA API request failed")
	}

	log.Infof("Issue %s updated successfully", issueIDOrKey)
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