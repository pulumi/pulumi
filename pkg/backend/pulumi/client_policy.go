// Copyright 2016-2020, Pulumi Corporation.
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

package pulumi

import (
	"context"
	"encoding/json"
	"io"

	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
)

func (c *Client) ListPolicyGroups(ctx context.Context, orgName string) (apitype.ListPolicyGroupsResponse, error) {
	return c.client.ListPolicyGroups(ctx, orgName)
}

func (c *Client) ListPolicyPacks(ctx context.Context, orgName string) (apitype.ListPolicyPacksResponse, error) {
	return c.client.ListPolicyPacks(ctx, orgName)
}

func (c *Client) GetPolicyPack(ctx context.Context, location string) ([]byte, error) {
	return c.client.DownloadPolicyPack(ctx, location)
}

func (c *Client) GetPolicyPackSchema(ctx context.Context, orgName, policyPackName,
	versionTag string) (*apitype.GetPolicyPackConfigSchemaResponse, error) {

	return c.client.GetPolicyPackSchema(ctx, orgName, policyPackName, versionTag)
}

func (c *Client) PublishPolicyPack(ctx context.Context, orgName string, analyzerInfo plugin.AnalyzerInfo,
	dirArchive io.Reader) (string, error) {

	return c.client.PublishPolicyPack(ctx, orgName, analyzerInfo, dirArchive)
}

func (c *Client) DeletePolicyPack(ctx context.Context, orgName, policyPackName, versionTag string) error {
	if versionTag == "" {
		return c.client.RemovePolicyPack(ctx, orgName, policyPackName)
	}
	return c.client.RemovePolicyPackByVersion(ctx, orgName, policyPackName, versionTag)
}

func (c *Client) EnablePolicyPack(ctx context.Context, orgName, policyGroup, policyPackName, versionTag string,
	policyPackConfig map[string]*json.RawMessage) error {

	return c.client.ApplyPolicyPack(ctx, orgName, policyGroup, policyPackName, versionTag, policyPackConfig)
}

func (c *Client) DisablePolicyPack(ctx context.Context, orgName, policyGroup, policyPackName, versionTag string) error {
	return c.client.DisablePolicyPack(ctx, orgName, policyGroup, policyPackName, versionTag)
}
