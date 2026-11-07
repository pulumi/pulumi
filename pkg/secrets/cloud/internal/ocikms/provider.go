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

package ocikms

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
)

func newGCConfigurationProvider(u *url.URL) (common.ConfigurationProvider, error) {
	switch os.Getenv("OCI_CLI_AUTH") {
	case "instance_principal":
		return auth.InstancePrincipalConfigurationProvider()
	case "", "api_key":
	default:
		return nil, errors.New("unsupported OCI_CLI_AUTH; expected api_key or instance_principal")
	}
	file := os.Getenv("OCI_CLI_CONFIG_FILE")
	if file == "" {
		file = os.Getenv("OCI_CONFIG_FILE")
	}
	profile := os.Getenv("OCI_CLI_PROFILE")
	if u != nil {
		if declared := u.Query().Get("conffile"); declared != "" {
			file = declared
		}
		if declared := u.Query().Get("profile"); declared != "" {
			profile = declared
		}
	}
	if file == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		file = filepath.Join(home, ".oci", "config")
	}
	if profile == "" {
		profile = "DEFAULT"
	}
	return common.ConfigurationProviderFromFileWithProfile(file, profile, os.Getenv("OCI_PRIVATE_KEY_PASSWORD"))
}
