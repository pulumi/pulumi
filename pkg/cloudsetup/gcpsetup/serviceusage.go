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

package gcpsetup

import (
	"context"
	"fmt"

	"google.golang.org/api/serviceusage/v1"
)

// serviceUsageClient is a thin wrapper around the GCP Service Usage API.
type serviceUsageClient interface {
	EnableService(ctx context.Context, projectID string, serviceName string) error
}

type realServiceUsageClient struct {
	serviceUsage *serviceusage.Service
}

func (c *realServiceUsageClient) EnableService(ctx context.Context, projectID string, serviceName string) error {
	name := fmt.Sprintf("projects/%s/services/%s", projectID, serviceName)
	_, err := c.serviceUsage.Services.Enable(name, &serviceusage.EnableServiceRequest{}).Context(ctx).Do()
	return err
}
