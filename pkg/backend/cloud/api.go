// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/backend/cloud/apitype"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

const (
	// defaultURL is the Cloud URL used if no environment or explicit cloud is chosen.
	defaultURL = "https://api.pulumi.com"
	// defaultAPIEnvVar can be set to override the default cloud chosen, if `--cloud` is not present.
	defaultURLEnvVar = "PULUMI_API"
	// AccessTokenEnvVar is the environment variable used to bypass a prompt on login.
	AccessTokenEnvVar = "PULUMI_ACCESS_TOKEN"
)

// DefaultURL returns the default cloud URL.  This may be overridden using the PULUMI_API environment
// variable.  If no override is found, the default is the pulumi.com cloud.
func DefaultURL() string {
	cloudURL := os.Getenv(defaultURLEnvVar)
	if cloudURL == "" {
		cloudURL = canonicalizeURL(defaultURL)
	}
	return cloudURL
}

// canonicalizeURL unifies cosmetic differences in a URL endpoint. We don't intend this to be 100% correct
// for arbitrary URLS. (e.g. handling "/a/b/../../c") Instead this is to safeguard against common mistakes
// in user configuration.
func canonicalizeURL(url string) string {
	url = strings.TrimSuffix(url, "/")
	return url
}

// cloudProjectIdentifier is the set of data needed to identify a Pulumi Cloud project. This the
// logical "home" of a stack on the Pulumi Cloud.
type cloudProjectIdentifier struct {
	Owner      string
	Repository string
	Project    tokens.PackageName
}

// pulumiAPICall makes an HTTP request to the Pulumi API.
func pulumiAPICall(apiEndpoint, method, path string, body []byte, accessToken string) (string, *http.Response, error) {
	// Normalize URL components
	apiEndpoint = strings.TrimSuffix(apiEndpoint, "/")
	path = strings.TrimPrefix(path, "/")

	url := fmt.Sprintf("%s/api/%s", apiEndpoint, path)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return "", nil, fmt.Errorf("creating new HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Apply credentials if provided.
	if accessToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", accessToken))
	}

	glog.V(7).Infof("Making Pulumi API call: %s", url)
	if glog.V(9) {
		glog.V(9).Infof("Pulumi API call details (%s): headers=%v; body=%v", url, req.Header, string(body))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("performing HTTP request: %v", err)
	}
	glog.V(7).Infof("Pulumi API call response code (%s): %v", url, resp.Status)

	return url, resp, nil
}

// pulumiRESTCall calls the pulumi REST API marshalling reqObj to JSON and using that as
// the request body (use nil for GETs), and if successful, marshalling the responseObj
// as JSON and storing it in respObj (use nil for NoContent). The error return type might
// be an instance of apitype.ErrorResponse, in which case will have the response code.
func pulumiRESTCall(cloudAPI, method, path string, reqObj interface{}, respObj interface{}) error {
	token, err := workspace.GetAccessToken(cloudAPI)
	if err != nil {
		return fmt.Errorf("getting stored credentials: %v", err)
	}
	return pulumiRESTCallWithAccessToken(cloudAPI, method, path, reqObj, respObj, token)
}

// pulumiRESTCallWithAccessToken requires you pass in the auth token rather than reading it from the machine's config.
func pulumiRESTCallWithAccessToken(cloudAPI, method, path string,
	reqObj interface{}, respObj interface{}, token string) error {
	var reqBody []byte
	var err error
	if reqObj != nil {
		reqBody, err = json.Marshal(reqObj)
		if err != nil {
			return fmt.Errorf("marshalling request object as JSON: %v", err)
		}
	}

	url, resp, err := pulumiAPICall(cloudAPI, method, path, reqBody, token)
	if err != nil {
		return fmt.Errorf("calling API: %v", err)
	}
	defer contract.IgnoreClose(resp.Body)

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from API: %v", err)
	}
	if glog.V(9) {
		glog.V(7).Infof("Pulumi API call response body (%s): %v", url, string(respBody))
	}

	// 4xx and 5xx responses should be of type ErrorResponse. See if we can unmarshal as that
	// type, and if not just return the raw response text.
	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		if resp.StatusCode == 401 {
			// Special case "unauthorized", and direct the developer to login.
			return errors.New("this command requires logging in; try running 'pulumi login' first")
		}

		var errResp apitype.ErrorResponse
		if err = json.Unmarshal(respBody, &errResp); err != nil {
			errResp.Code = resp.StatusCode
			errResp.Message = strings.TrimSpace(string(respBody))
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
