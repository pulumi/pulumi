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

package plugin

import (
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// ErrorKind classifies a failure that a provider reports, so that callers can react to it without parsing its message.
type ErrorKind int

const (
	// ErrorKindUnspecified means the provider did not say what kind of failure this is.
	ErrorKindUnspecified ErrorKind = 0
	// ErrorKindCredentials means the provider could not authenticate with its cloud or service, for example because
	// credentials are missing, invalid or expired.
	ErrorKindCredentials ErrorKind = 1
)

func errorKindFromRPC(k pulumirpc.ErrorKind) ErrorKind {
	switch k {
	case pulumirpc.ErrorKind_ERROR_KIND_CREDENTIALS:
		return ErrorKindCredentials
	case pulumirpc.ErrorKind_ERROR_KIND_UNSPECIFIED:
		return ErrorKindUnspecified
	default:
		// A kind added in a newer protocol version than this SDK knows.
		return ErrorKindUnspecified
	}
}

func (k ErrorKind) rpc() pulumirpc.ErrorKind {
	switch k {
	case ErrorKindCredentials:
		return pulumirpc.ErrorKind_ERROR_KIND_CREDENTIALS
	case ErrorKindUnspecified:
		return pulumirpc.ErrorKind_ERROR_KIND_UNSPECIFIED
	default:
		return pulumirpc.ErrorKind_ERROR_KIND_UNSPECIFIED
	}
}
