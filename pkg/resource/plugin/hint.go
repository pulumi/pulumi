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

// Hint names a product hint that is relevant to a failure a provider reports. The provider says which hint is
// relevant and what it says, and the engine decides whether to show it.
type Hint int

const (
	// HintUnspecified means that no product hint is relevant.
	HintUnspecified Hint = 0
	// HintDynamicCredentials means that the failure comes from missing, rejected or expired credentials, so short-lived
	// credentials from an identity provider, for example through OIDC, may help.
	HintDynamicCredentials Hint = 1
)

func hintFromRPC(h pulumirpc.Hint) Hint {
	switch h {
	case pulumirpc.Hint_HINT_DYNAMIC_CREDENTIALS:
		return HintDynamicCredentials
	case pulumirpc.Hint_HINT_UNSPECIFIED:
		return HintUnspecified
	default:
		// A hint added in a newer protocol version than this engine knows.
		return HintUnspecified
	}
}

func (h Hint) rpc() pulumirpc.Hint {
	switch h {
	case HintDynamicCredentials:
		return pulumirpc.Hint_HINT_DYNAMIC_CREDENTIALS
	case HintUnspecified:
		return pulumirpc.Hint_HINT_UNSPECIFIED
	default:
		return pulumirpc.Hint_HINT_UNSPECIFIED
	}
}

// HintRecorder is told about each hint that a provider reports for a failure. Set it on [Context.HintRecorder].
type HintRecorder interface {
	// RecordHint is told about hint and the message the provider sent for it.
	RecordHint(hint Hint, message string)
}

// recordHint reports hint and its message to the hint recorder of the provider's plugin context, if there is one.
func (p *provider) recordHint(hint Hint, message string) {
	if hint == HintUnspecified || p.ctx == nil || p.ctx.HintRecorder == nil {
		return
	}
	p.ctx.HintRecorder.RecordHint(hint, message)
}
