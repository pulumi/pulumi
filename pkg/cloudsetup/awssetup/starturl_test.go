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

package awssetup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateSSOStartURL(t *testing.T) {
	t.Parallel()

	valid := []string{
		"https://d-1234567890.awsapps.com/start",
		"https://d-1234567890.awsapps.com/start/",
		"https://d-1234567890.awsapps.com/start#/",
		"https://d-1234567890.awsapps.com",
		"https://my-org.awsapps.com/start?foo=bar",
		"https://start.us-gov-east-1.us-gov-home.awsapps.com/directory/d-1234567890",
		"https://d-1234567890.awsapps.cn/start",
		"https://portal.sso.us-east-1.amazonaws.com/start",
		"HTTPS://D-1234567890.AWSAPPS.COM/START",
		"  https://d-1234567890.awsapps.com/start  ",
	}
	for _, u := range valid {
		t.Run("valid "+u, func(t *testing.T) {
			assert.NoError(t, ValidateSSOStartURL(u))
		})
	}

	invalid := []string{
		"",
		"   ",
		"d-1234567890.awsapps.com/start",
		"http://d-1234567890.awsapps.com/start",
		"https://awsapps.com/start",
		"https://.awsapps.com/start",
		"https://d-1234567890.awsapps.co/start",
		"https://d-1234567890.awsapps.com.evil.example/start",
		"https://example.com/start",
		"https://d-1234567890.awsapps.com/start with space",
		"not a url",
	}
	for _, u := range invalid {
		t.Run("invalid "+u, func(t *testing.T) {
			assert.ErrorIs(t, ValidateSSOStartURL(u), ErrInvalidSSOStartURL)
		})
	}
}
