// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCheckHTTPCloudBackendUrlWithAppPulumi calls checkHTTPCloudBackenUrl with a
// wrong pulumi cloud url, checking for a valid return value.
func TestCheckHTTPCloudBackendUrlWithAppPulumi(t *testing.T) {
	t.Parallel()

	cloudURL := "https://app.pulumi.com"
	_, err := checkHTTPCloudBackendURL(cloudURL)

	require.Error(t, fmt.Errorf("%s is not a valid self-hosted backend, "+
		"use `pulumi login` without arguments to log into the Pulumi Cloud backend", cloudURL), err)
}
