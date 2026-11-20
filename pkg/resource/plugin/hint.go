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

// Hint is a product hint, which points the user to a Pulumi feature that helps with a failure a provider reports.
// The provider writes its text, and the engine decides whether to show it.
type Hint struct {
	// Kind says which product hint this is.
	Kind HintKind
	// Message is the text that the engine shows if it decides to show the hint.
	Message string
}

// HintKind names a product hint.
type HintKind int

const (
	// HintUnspecified means that no product hint is relevant.
	HintUnspecified HintKind = 0
	// HintDynamicCredentials means that the failure comes from missing, rejected or expired credentials, so short-lived
	// credentials from an identity provider, for example through OIDC, may help.
	HintDynamicCredentials HintKind = 1
)

func hintFromRPC(h *pulumirpc.Hint) *Hint {
	if h == nil {
		return nil
	}
	var kind HintKind
	switch h.GetKind() {
	case pulumirpc.Hint_DYNAMIC_CREDENTIALS:
		kind = HintDynamicCredentials
	case pulumirpc.Hint_UNSPECIFIED:
		kind = HintUnspecified
	default:
		// A hint added in a newer protocol version than this engine knows.
		kind = HintUnspecified
	}
	return &Hint{Kind: kind, Message: h.GetMessage()}
}

func (h *Hint) rpc() *pulumirpc.Hint {
	if h == nil {
		return nil
	}
	var kind pulumirpc.Hint_Kind
	switch h.Kind {
	case HintDynamicCredentials:
		kind = pulumirpc.Hint_DYNAMIC_CREDENTIALS
	case HintUnspecified:
		kind = pulumirpc.Hint_UNSPECIFIED
	default:
		kind = pulumirpc.Hint_UNSPECIFIED
	}
	return &pulumirpc.Hint{Kind: kind, Message: h.Message}
}

// HintRecorder is told about each hint that a provider reports for a failure. Set it on [Context.HintRecorder].
type HintRecorder interface {
	// RecordHint is told about a hint the provider sent.
	RecordHint(hint Hint)
}

// recordHint reports hint to the hint recorder of the provider's plugin context, if there is one.
func (p *provider) recordHint(hint *Hint) {
	if hint == nil || hint.Kind == HintUnspecified || p.ctx == nil || p.ctx.HintRecorder == nil {
		return
	}
	p.ctx.HintRecorder.RecordHint(*hint)
}
