// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strconv"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// PublishPolicyPack publishes a `PolicyPack` to the Pulumi service: it creates the new
// version, uploads the source archive and any per-platform binary artifacts to the
// presigned locations the service returns, and signals completion. It returns the
// version of the published pack.
func (pc *Client) PublishPolicyPack(ctx context.Context, orgName string,
	runtime string, analyzerInfo plugin.AnalyzerInfo, sourceTarball []byte,
	platformArchives map[string][]byte, metadata map[string]string,
) (string, error) {
	//
	// Step 1: Send POST containing policy metadata to service. This begins process of creating
	// publishing the PolicyPack.
	//

	// An empty version is tolerated for source-only packs: it means an older version of
	// pulumi/policy that does not gather the version, in which case the service assigns one.
	if analyzerInfo.Version == "" && len(platformArchives) > 0 {
		return "", errors.New("a version is required to publish a policy pack with binaries")
	}
	if err := validatePolicyPackVersion(analyzerInfo.Version); err != nil {
		return "", err
	}

	platforms := slices.Sorted(maps.Keys(platformArchives))
	req, err := buildCreatePolicyPackRequest(runtime, analyzerInfo, metadata, platforms)
	if err != nil {
		return "", err
	}

	var versionMsg string
	if analyzerInfo.Version != "" {
		versionMsg = " - version " + analyzerInfo.Version
	}
	fmt.Printf("Publishing %q%s to %q\n", analyzerInfo.Name, versionMsg, orgName)

	var resp apitype.CreatePolicyPackResponse
	if err := pc.restCall(ctx, "POST", publishPolicyPackPath(orgName), nil, req, &resp); err != nil {
		return "", fmt.Errorf("Publish policy pack failed: %w", err)
	}

	if len(platforms) > 0 && len(resp.PlatformUploadURIs) == 0 {
		return "", errors.New(
			"this Pulumi service version does not support policy pack binaries; " +
				"publish without the pre-built binaries to publish source only")
	}

	//
	// Step 2: Upload the compressed PolicyPack directory and any per-platform binary
	// artifacts to the pre-signed object storage service URLs.
	//

	if err := pc.uploadPolicyPackArtifact(
		resp.UploadURI, resp.RequiredHeaders, sourceTarball, "source"); err != nil {
		return "", err
	}
	for _, platform := range platforms {
		upload, ok := resp.PlatformUploadURIs[platform]
		if !ok {
			return "", fmt.Errorf("the service did not return an upload location for platform %s", platform)
		}
		if err := pc.uploadPolicyPackArtifact(
			upload.UploadURI, upload.RequiredHeaders, platformArchives[platform], platform); err != nil {
			return "", err
		}
	}

	//
	// Step 3: Signal to the service that the PolicyPack publish operation is complete.
	//

	// If the version tag is empty, an older version of pulumi/policy is being used and
	// we therefore need to use the version provided by the pulumi service.
	version := analyzerInfo.Version
	if version == "" {
		version = strconv.Itoa(resp.Version)
		fmt.Printf("Published as version %s\n", version)
	}
	if err := pc.restCall(ctx, "POST",
		publishPolicyPackPublishComplete(orgName, analyzerInfo.Name, version), nil, nil, nil); err != nil {
		return "", fmt.Errorf("Request to signal completion of the publish operation failed: %w", err)
	}

	return version, nil
}

// buildCreatePolicyPackRequest converts an analyzer's policy metadata into the wire request used
// to create a new PolicyPack version. platforms is non-empty only for packs that publish
// per-platform binary artifacts in addition to the source archive.
func buildCreatePolicyPackRequest(
	runtime string, analyzerInfo plugin.AnalyzerInfo, metadata map[string]string, platforms []string,
) (apitype.CreatePolicyPackRequest, error) {
	policies := make([]apitype.Policy, len(analyzerInfo.Policies))
	for i, policy := range analyzerInfo.Policies {
		configSchema, err := convertPolicyConfigSchema(policy.ConfigSchema)
		if err != nil {
			return apitype.CreatePolicyPackRequest{}, err
		}

		policies[i] = apitype.Policy{
			Name:             policy.Name,
			DisplayName:      policy.DisplayName,
			Description:      policy.Description,
			EnforcementLevel: policy.EnforcementLevel,
			Message:          policy.Message,
			ConfigSchema:     configSchema,
			Severity:         policy.Severity,
			Framework:        convertPolicyComplianceFramework(policy.Framework),
			Tags:             policy.Tags,
			RemediationSteps: policy.RemediationSteps,
			URL:              policy.URL,
		}
	}

	return apitype.CreatePolicyPackRequest{
		Name:        analyzerInfo.Name,
		DisplayName: analyzerInfo.DisplayName,
		VersionTag:  analyzerInfo.Version,
		Policies:    policies,
		Description: analyzerInfo.Description,
		Readme:      analyzerInfo.Readme,
		Provider:    analyzerInfo.Provider,
		Tags:        analyzerInfo.Tags,
		Repository:  analyzerInfo.Repository,
		Runtime:     runtime,
		Metadata:    metadata,
		Platforms:   platforms,
	}, nil
}

// uploadPolicyPackArtifact uploads a single artifact (the source archive or one platform's
// binary tarball) to its presigned URL. what names the artifact in errors.
func (pc *Client) uploadPolicyPackArtifact(
	uploadURI string, requiredHeaders map[string]string, artifact []byte, what string,
) error {
	putReq, err := http.NewRequest(http.MethodPut, uploadURI, bytes.NewReader(artifact))
	if err != nil {
		return fmt.Errorf("failed to upload policy pack artifact for %s: %w", what, err)
	}
	for k, v := range requiredHeaders {
		putReq.Header.Add(k, v)
	}
	resp, err := pc.restClient.HTTPClient().Do(putReq, retryAllMethods)
	if err != nil {
		return fmt.Errorf("failed to upload policy pack artifact for %s: %w", what, err)
	}
	contract.IgnoreClose(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to upload policy pack artifact for %s: upload returned status %s",
			what, resp.Status)
	}
	return nil
}

// convertPolicyConfigSchema converts a policy's schema from the analyzer to the apitype.
func convertPolicyConfigSchema(schema *plugin.AnalyzerPolicyConfigSchema) (*apitype.PolicyConfigSchema, error) {
	if schema == nil {
		return nil, nil
	}
	properties := map[string]*json.RawMessage{}
	for k, v := range schema.Properties {
		bytes, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		raw := json.RawMessage(bytes)
		properties[k] = &raw
	}
	return &apitype.PolicyConfigSchema{
		Type:       apitype.Object,
		Properties: properties,
		Required:   schema.Required,
	}, nil
}

// convertPolicyComplianceFramework converts a policy compliance framework from the analyzer to the apitype.
func convertPolicyComplianceFramework(f *plugin.AnalyzerPolicyComplianceFramework) *apitype.PolicyComplianceFramework {
	if f == nil {
		return nil
	}
	return &apitype.PolicyComplianceFramework{
		Name:          f.Name,
		Version:       f.Version,
		Reference:     f.Reference,
		Specification: f.Specification,
	}
}

// validatePolicyPackVersion validates the version of a Policy Pack. The version may be empty,
// as it is likely an older version of pulumi/policy that does not gather the version.
func validatePolicyPackVersion(s string) error {
	if s == "" {
		return nil
	}

	policyPackVersionTagRE := regexp.MustCompile("^[a-zA-Z0-9-_.]{1,100}$")
	if !policyPackVersionTagRE.MatchString(s) {
		msg := fmt.Sprintf("invalid version %q - version may only contain alphanumeric, hyphens, or underscores. "+
			"It must also be between 1 and 100 characters long.", s)
		return errors.New(msg)
	}
	return nil
}
