package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	envutil "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

func AIEnabled() bool {
	return envutil.BoolValue(env.AIErrorSuggestions).Value()
}

func NewPulumiAIClient() PulumiAIClient {
	return PulumiAIClientConfig{
		// Language:      "",
		// CloudProvider: "",
		APIEndpoint: envutil.StringValue(env.AIServiceEndpoint).Value(),
	}
}

type PulumiAIClientConfig struct {
	// Language      string
	// CloudProvider string
	APIEndpoint string
}

type PulumiAIClient interface {
	SendErrorMessageHelpRequest(errorMessage string) (suggestion []string, err error)
	SendHealthCheckRequest() (err error)
}

type PulumiErrorMessageHelpRequest struct {
	ErrorMessage string `json:"errorMessage"`
}

type PulumiErrorMessageHelpResponse struct {
	Suggestion []string `json:"suggestion"`
}

func (config PulumiAIClientConfig) SendHealthCheckRequest() (err error) {
	res, err := http.Get(fmt.Sprintf("%s/health", config.APIEndpoint))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %s", res.Status)
	}
	return nil
}

func (config PulumiAIClientConfig) SendErrorMessageHelpRequest(errorMessage string) (suggestion []string, err error) {
	errorMessageBody := PulumiErrorMessageHelpRequest{
		ErrorMessage: errorMessage,
	}
	requestBody, err := json.Marshal(errorMessageBody)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/pulumiErrorPrompt", config.APIEndpoint), bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	structuredResponse := PulumiErrorMessageHelpResponse{}
	err = json.Unmarshal(body, &structuredResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}
	return structuredResponse.Suggestion, nil
}
