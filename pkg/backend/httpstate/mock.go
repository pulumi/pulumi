// Copyright 2016-2023, Pulumi Corporation.
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

package httpstate

import (
	"context"
	"net/http"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type MockHTTPBackend struct {
	backend.MockBackend
	FClient   func() *client.Client
	FCloudURL func() string
	FSearch   func(ctx context.Context,
		orgName string,
		queryParams *apitype.PulumiQueryRequest,
	) (*apitype.ResourceSearchResponse, error)
	FNaturalLanguageSearch func(ctx context.Context, orgName string, query string) (*apitype.ResourceSearchResponse, error)
	FPromptAI              func(ctx context.Context, requestBody AIPromptRequestBody) (*http.Response, error)
	FStackConsoleURL       func(stackRef backend.StackReference) (string, error)
	FRunDeployment         func(
		ctx context.Context,
		stackRef backend.StackReference,
		req apitype.CreateDeploymentRequest,
		opts display.Options,
		deploymentInitiator string,
		suppressStreamLogs bool,
	) error
}

func (b *MockHTTPBackend) Client() *client.Client {
	return b.FClient()
}

func (b *MockHTTPBackend) CloudURL() string {
	return b.FCloudURL()
}

func (b *MockHTTPBackend) NaturalLanguageSearch(
	ctx context.Context, orgName string, query string,
) (*apitype.ResourceSearchResponse, error) {
	return b.FNaturalLanguageSearch(ctx, orgName, query)
}

func (b *MockHTTPBackend) PromptAI(
	ctx context.Context, requestBody AIPromptRequestBody,
) (*http.Response, error) {
	return b.FPromptAI(ctx, requestBody)
}

func (b *MockHTTPBackend) StackConsoleURL(stackRef backend.StackReference) (string, error) {
	return b.FStackConsoleURL(stackRef)
}

func (b *MockHTTPBackend) RunDeployment(
	ctx context.Context,
	stackRef backend.StackReference,
	req apitype.CreateDeploymentRequest,
	opts display.Options,
	deploymentInitiator string,
	suppressStreamLogs bool,
) error {
	return b.FRunDeployment(ctx, stackRef, req, opts, deploymentInitiator, suppressStreamLogs)
}

func (b *MockHTTPBackend) Search(
	ctx context.Context, orgName string, queryParams *apitype.PulumiQueryRequest,
) (*apitype.ResourceSearchResponse, error) {
	return b.FSearch(ctx, orgName, queryParams)
}

func (b *MockHTTPBackend) Capabilities(context.Context) apitype.Capabilities {
	return apitype.Capabilities{}
}

var _ Backend = (*MockHTTPBackend)(nil)
