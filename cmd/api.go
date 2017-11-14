// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// pulumiCloudEndpoint returns the endpoint for the Pulumi Cloud Management Console API.
// e.g. "http://localhost:8080" or "https://api.moolumi.io".
func pulumiConsoleAPI() (string, error) {
	envVar := os.Getenv("PULUMI_API")
	if envVar == "" {
		return "", fmt.Errorf("PULUMI_API env var not set")
	}
	return strings.TrimSuffix(envVar, "/"), nil
}

// usePulumiCloudCommands returns whether or not to use the "Pulumi Cloud" version of CLI commands.
func usePulumiCloudCommands() bool {
	_, err := pulumiConsoleAPI()
	return err == nil
}

// cloudProjectIdentifier is the set of data needed to identify a Pulumi Cloud project. This the
// logical "home" of a stack on the Pulumi Cloud.
type cloudProjectIdentifier struct {
	Owner      string
	Repository string
	Project    tokens.PackageName
}

// pulumiAPICall makes an HTTP request to the Pulumi API.
func pulumiAPICall(method string, path string, body []byte, accessToken string) (*http.Response, error) {
	apiEndpoint, err := pulumiConsoleAPI()
	if err != nil {
		return nil, fmt.Errorf("getting Pulumi API endpoint: %v", err)
	}

	// Normalize URL components
	apiEndpoint = strings.TrimSuffix(apiEndpoint, "/")
	path = strings.TrimPrefix(path, "/")

	url := fmt.Sprintf("%s/api/%s", apiEndpoint, path)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("creating new HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Apply credentials if provided.
	if accessToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", accessToken))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing HTTP request: %v", err)
	}
	return resp, nil
}

// pulumiRESTCall calls the pulumi REST API marshalling reqObj to JSON and using that as
// the request body (use nil for GETs), and if successful, marshalling the responseObj
// as JSON and storing it in respObj (use nil for NoContent). The error return type might
// be an instance of apitype.ErrorResponse, in which case will have the response code.
func pulumiRESTCall(method, path string, reqObj interface{}, respObj interface{}) error {
	creds, err := getStoredCredentials()
	if err != nil {
		return fmt.Errorf("getting stored credentials: %v", err)
	}
	return pulumiRESTCallWithAccessToken(method, path, reqObj, respObj, creds.AccessToken)
}

// pulumiRESTCallWithAccessToken requires you pass in the auth token rather than reading it from the machine's config.
func pulumiRESTCallWithAccessToken(method, path string, reqObj interface{}, respObj interface{}, token string) error {
	var reqBody []byte
	var err error
	if reqObj != nil {
		reqBody, err = json.Marshal(reqObj)
		if err != nil {
			return fmt.Errorf("marshalling request object as JSON: %v", err)
		}
	}

	resp, err := pulumiAPICall(method, path, reqBody, token)
	if err != nil {
		return fmt.Errorf("calling API: %v", err)
	}
	defer contract.IgnoreClose(resp.Body)
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from API: %v", err)
	}

	// 4xx and 5xx responses should be of type ErrorResponse. See if we can unmarshal as that
	// type, and if not just return the raw response text.
	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		var errResp apitype.ErrorResponse
		if err = json.Unmarshal(respBody, &errResp); err != nil {
			errResp.Code = resp.StatusCode
			errResp.Message = string(respBody)
		}
		return &errResp
	}

	if respObj != nil {
		if err = json.Unmarshal(respBody, respObj); err != nil {
			return fmt.Errorf("unmarshalling response object: %v", err)
		}
	}

	return nil
}
