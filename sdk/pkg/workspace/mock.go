// Copyright 2016-2024, Pulumi Corporation.
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

package workspace

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type MockContext struct {
	ReadProjectF          func() (*workspace.Project, string, error)
	GetStoredCredentialsF func() (workspace.Credentials, error)
}

func (c *MockContext) ReadProject() (*workspace.Project, string, error) {
	if c.ReadProjectF != nil {
		return c.ReadProjectF()
	}
	return nil, "", workspace.ErrProjectNotFound
}

func (c *MockContext) GetStoredCredentials() (workspace.Credentials, error) {
	if c.GetStoredCredentialsF != nil {
		return c.GetStoredCredentialsF()
	}
	return workspace.Credentials{}, nil
}
