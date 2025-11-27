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

package deploy

import (
	"errors"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestEnhanceAlreadyExistsError(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:stack::project::aws:s3/bucket:Bucket::my-bucket")

	tests := []struct {
		name          string
		inputError    error
		shouldEnhance bool
		expectedText  []string
	}{
		{
			name:          "AWS EntityAlreadyExists error",
			inputError:    errors.New("creating IAM User (test-user): EntityAlreadyExists: User with name test-user already exists.\nstatus code: 409, request id: 12345"),
			shouldEnhance: true,
			expectedText:  []string{"pulumi import", "aws:s3/bucket:Bucket", "my-bucket", "<resource-id>", "To resolve this error"},
		},
		{
			name:          "Generic already exists error",
			inputError:    errors.New("resource already exists in the provider"),
			shouldEnhance: true,
			expectedText:  []string{"pulumi import", "To resolve this error"},
		},
		{
			name:          "Conflict error",
			inputError:    errors.New("Error creating resource: Conflict"),
			shouldEnhance: true,
			expectedText:  []string{"pulumi import", "To resolve this error"},
		},
		{
			name:          "Status code 409 error",
			inputError:    errors.New("HTTP request failed with status code: 409"),
			shouldEnhance: true,
			expectedText:  []string{"pulumi import", "To resolve this error"},
		},
		{
			name:          "AlreadyExists error",
			inputError:    errors.New("AlreadyExists: resource cannot be created"),
			shouldEnhance: true,
			expectedText:  []string{"pulumi import", "To resolve this error"},
		},
		{
			name:          "Unrelated error - should not enhance",
			inputError:    errors.New("invalid property value"),
			shouldEnhance: false,
			expectedText:  []string{"invalid property value"},
		},
		{
			name:          "Permission denied error - should not enhance",
			inputError:    errors.New("access denied: insufficient permissions"),
			shouldEnhance: false,
			expectedText:  []string{"access denied"},
		},
		{
			name:          "Nil error",
			inputError:    nil,
			shouldEnhance: false,
			expectedText:  nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := enhanceAlreadyExistsError(tt.inputError, urn)

			if tt.inputError == nil {
				assert.Nil(t, result)
				return
			}

			assert.NotNil(t, result)
			resultMsg := result.Error()

			if tt.shouldEnhance {
				// Verify the enhanced message contains all expected texts
				for _, expectedText := range tt.expectedText {
					assert.Contains(t, resultMsg, expectedText, "Enhanced error should contain: %s", expectedText)
				}

				// Verify it contains the original error message
				assert.Contains(t, resultMsg, tt.inputError.Error(), "Enhanced error should contain original message")

				// Verify it contains the three resolution steps
				assert.Contains(t, resultMsg, "1. Import the resource:")
				assert.Contains(t, resultMsg, "2. Change the resource name:")
				assert.Contains(t, resultMsg, "3. Delete the external resource:")
			} else {
				// Should return the original error unchanged
				assert.Equal(t, tt.inputError.Error(), resultMsg)
			}
		})
	}
}

func TestEnhanceAlreadyExistsError_CaseInsensitive(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:stack::project::aws:ec2/instance:Instance::my-instance")

	// Test case insensitivity
	variations := []string{
		"Already Exists",
		"ALREADY EXISTS",
		"AlreadyExists",
		"already exists",
		"EntityAlreadyExists",
		"ENTITYALREADYEXISTS",
	}

	for _, variation := range variations {
		t.Run(variation, func(t *testing.T) {
			err := errors.New("Error: " + variation)
			result := enhanceAlreadyExistsError(err, urn)

			assert.NotNil(t, result)
			assert.Contains(t, result.Error(), "To resolve this error")
			assert.Contains(t, result.Error(), "pulumi import")
		})
	}
}

func TestEnhanceAlreadyExistsError_PreservesOriginalMessage(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:stack::project::gcp:compute/instance:Instance::vm")
	originalMsg := "creating GCP Instance: Resource already exists with ID 'instance-123'. Status code: 409"

	err := errors.New(originalMsg)
	result := enhanceAlreadyExistsError(err, urn)

	assert.NotNil(t, result)
	resultMsg := result.Error()

	// Original message should be at the beginning
	assert.True(t, strings.HasPrefix(resultMsg, originalMsg), "Enhanced error should start with original message")

	// Followed by helpful guidance
	assert.Contains(t, resultMsg, "\n\nTo resolve this error")
}
