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

package awssetuptypes

type Config struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken,omitempty"`
	Region          string `json:"region"`
}

type SSOConfig struct {
	VerificationURL string `json:"verificationUrl"`
	ClientID        string `json:"clientId"`
	ClientSecret    string `json:"clientSecret"`
	DeviceCode      string `json:"deviceCode"`
	Interval        int32  `json:"interval"`
	UserCode        string `json:"userCode"`
	ExpiresIn       int32  `json:"expiresIn"`
}
