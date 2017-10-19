package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/pulumi/pulumi-service/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// pulumiCloudEndpoint returns the endpoint for the Pulumi Cloud Management Console API.
// e.g. "http://localhost:8080" or "https://api.moolumi.io".
func pulumiConsoleAPI() (string, error) {
	envVar := os.Getenv("PULUMI_API")
	if envVar == "" {
		return "", fmt.Errorf("PULUMI_API env var not set")
	}
	return envVar, nil
}

// pulumiAPICall makes an HTTP request to the Pulumi API.
func pulumiAPICall(method string, path string, body []byte) (*http.Response, error) {
	apiEndpoint, err := pulumiConsoleAPI()
	if err != nil {
		return nil, fmt.Errorf("getting Pulumi API endpoint: %v", err)
	}

	url := fmt.Sprintf("%s/api%s", apiEndpoint, path)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("creating new HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Apply stored credentials if possible.
	creds, _ := GetStoredCredentials()
	if creds != nil {
		req.Header.Set("Authorization", "token "+creds.AccessToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing HTTP request: %v", err)
	}
	return resp, nil
}

// PulumiRESTCall calls the pulumi REST API marshalling reqObj to JSON and using that as the
// request body (use nil for GETs), and if successful, marshalling the responseObj as JSON and
// storing it in respObj (use nil for NoContent).
//
// If the API request is made successfully, but the API returns an error, the ErrorResponse
// valuewill be returned. error will be returned for any other type of IO error.
func PulumiRESTCall(method, path string, reqObj interface{}, respObj interface{}) (*apitype.ErrorResponse, error) {
	var reqBody []byte
	var err error
	if reqObj != nil {
		reqBody, err = json.Marshal(reqObj)
		if err != nil {
			return nil, fmt.Errorf("marshalling request object as JSON: %v", err)
		}
	}

	resp, err := pulumiAPICall(method, path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("calling API: %v", err)
	}
	defer contract.IgnoreClose(resp.Body)
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from API: %v", err)
	}

	// 4xx and 5xx responses should be of type ErrorResponse. See if we can unmarshal as that
	// type, and if not just return the raw response text.
	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		var errResp apitype.ErrorResponse
		if err = json.Unmarshal(respBody, &errResp); err != nil {
			return nil, fmt.Errorf("response error: %v", string(respBody))
		}
		return &errResp, nil
	}

	if respObj != nil {
		if err = json.Unmarshal(respBody, respObj); err != nil {
			return nil, fmt.Errorf("unmarshalling response object: %v", err)
		}
	}

	return nil, nil
}
