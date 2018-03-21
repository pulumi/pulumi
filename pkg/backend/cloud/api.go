// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/golang/glog"
	"github.com/google/go-querystring/query"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/version"
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
// variable.  If no override is found, and we are authenticated with only one cloud, choose that.  Otherwise,
// we will default to the https://api.pulumi.com/ endpoint.
func DefaultURL() string {
	return ValueOrDefaultURL("")
}

// ValueOrDefaultURL returns the value if specified, or the default cloud URL otherwise.
func ValueOrDefaultURL(cloudURL string) string {
	// If we have a cloud URL, just return it.
	if cloudURL != "" {
		return cloudURL
	}

	// Otherwise, respect the PULUMI_API override.
	if cloudURL := os.Getenv(defaultURLEnvVar); cloudURL != "" {
		return cloudURL
	}

	// If that didn't work, see if we're authenticated with any clouds.
	urls, current, err := CurrentBackendURLs()
	if err == nil {
		if current != "" {
			// If there's a current cloud selected, return that.
			return current
		} else if len(urls) == 1 {
			// Else, if we're authenticated with a single cloud, use that.
			return urls[0]
		}
	}

	// If none of those led to a cloud URL, simply return the default.
	return defaultURL
}

// cloudProjectIdentifier is the set of data needed to identify a Pulumi Cloud project. This the
// logical "home" of a stack on the Pulumi Cloud.
type cloudProjectIdentifier struct {
	Owner      string
	Repository string
	Project    tokens.PackageName
}

// pulumiAPICall makes an HTTP request to the Pulumi API.
func pulumiAPICall(cloudAPI, method, path string, body []byte) (string, *http.Response, error) {
	accessToken, err := workspace.GetAccessToken(cloudAPI)
	if err != nil {
		return "", nil, errors.Wrapf(err, "getting stored credentials")
	}
	return pulumiAPICallToken(cloudAPI, method, path, body, accessToken)
}

// pulumiAPICallToken makes an HTTP request to the Pulumi API with an explicit access token.
func pulumiAPICallToken(cloudAPI, method, path string, body []byte,
	accessToken string) (string, *http.Response, error) {
	// Normalize URL components
	cloudAPI = strings.TrimSuffix(cloudAPI, "/")
	path = strings.TrimPrefix(path, "/")

	url := fmt.Sprintf("%s/%s", cloudAPI, path)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return "", nil, errors.Wrapf(err, "creating new HTTP request")
	}
	req.Header.Set("Content-Type", "application/json")

	// Add a User-Agent header to allow for the backend to make breaking API changes while preserving
	// backwards compatibility.
	userAgent := fmt.Sprintf("pulumi-cli/1 (%s; %s)", version.Version, runtime.GOOS)
	req.Header.Set("User-Agent", userAgent)

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
		return "", nil, errors.Wrapf(err, "performing HTTP request")
	}
	glog.V(7).Infof("Pulumi API call response code (%s): %v", url, resp.Status)

	// For 4xx and 5xx failures, attempt to provide better diagnostics about what may have gone wrong.
	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		defer contract.IgnoreClose(resp.Body)

		// Provide a better error if using an authenticated call without having logged in first.
		if resp.StatusCode == 401 && accessToken == "" {
			return "", nil, errors.New("this command requires logging in; try running 'pulumi login' first")
		}

		// 4xx and 5xx responses should be of type ErrorResponse. See if we can unmarshal as that
		// type, and if not just return the raw response text.
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", nil, errors.Wrapf(
				err, "API call failed (%s), could not read response", resp.Status)
		}

		var errResp apitype.ErrorResponse
		if err = json.Unmarshal(respBody, &errResp); err != nil {
			errResp.Code = resp.StatusCode
			errResp.Message = strings.TrimSpace(string(respBody))
		}
		return "", nil, &errResp
	}

	return url, resp, nil
}

// pulumiRESTCall calls the Pulumi REST API marshalling reqObj to JSON and using that as
// the request body (use nil for GETs), and if successful, marshalling the responseObj
// as JSON and storing it in respObj (use nil for NoContent). The error return type might
// be an instance of apitype.ErrorResponse, in which case will have the response code.
func pulumiRESTCall(cloudAPI, method, path string,
	queryObj interface{}, reqObj interface{}, respObj interface{}) error {
	accessToken, err := workspace.GetAccessToken(cloudAPI)
	if err != nil {
		return errors.Wrapf(err, "getting stored credentials")
	}
	return pulumiRESTCallToken(cloudAPI, method, path, queryObj, reqObj, respObj, accessToken)
}

// pulumiRESTCallToken calls the Pulumi REST API, just like pulumiRESTCall, only with
// an explicit accessToken rather than reading it from the workspace.
func pulumiRESTCallToken(cloudAPI, method, path string,
	queryObj interface{}, reqObj interface{}, respObj interface{}, accessToken string) error {
	// Compute query string from query object
	querystring := ""
	if queryObj != nil {
		queryValues, err := query.Values(queryObj)
		if err != nil {
			return errors.Wrapf(err, "marshalling query object as JSON")
		}
		query := queryValues.Encode()
		if len(query) > 0 {
			querystring = "?" + query
		}
	}

	// Compute request body from request object
	var reqBody []byte
	var err error
	if reqObj != nil {
		reqBody, err = json.Marshal(reqObj)
		if err != nil {
			return errors.Wrapf(err, "marshalling request object as JSON")
		}
	}

	// Make API call
	url, resp, err := pulumiAPICallToken(cloudAPI, method, path+querystring, reqBody, accessToken)
	if err != nil {
		return errors.Wrapf(err, "calling API")
	}
	defer contract.IgnoreClose(resp.Body)

	// Read API response
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "reading response from API")
	}
	if glog.V(9) {
		glog.V(7).Infof("Pulumi API call response body (%s): %v", url, string(respBody))
	}

	if respObj != nil {
		if err = json.Unmarshal(respBody, respObj); err != nil {
			return errors.Wrapf(err, "unmarshalling response object")
		}
	}

	return nil
}
