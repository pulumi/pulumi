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
	"context"
	"errors"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Hint names a product hint that is relevant to a failure a provider reports. The provider says which hint is
// relevant, and the engine decides whether to show it.
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

// HintOf returns the hint that a provider attached to err as a HintDetail. It returns HintUnspecified for errors
// without such a detail.
func HintOf(err error) Hint {
	if err == nil {
		return HintUnspecified
	}
	rpcErr, ok := rpcerror.FromError(err)
	if !ok {
		if rpcErr, ok = errors.AsType[*rpcerror.Error](err); !ok {
			return HintUnspecified
		}
	}
	for _, d := range rpcErr.Details() {
		if detail, ok := d.(*pulumirpc.HintDetail); ok {
			if hint := hintFromRPC(detail.GetHint()); hint != HintUnspecified {
				return hint
			}
		}
	}
	return HintUnspecified
}

// WithHint attaches a HintDetail to err, which must be a gRPC status error such as one made by
// [rpcerror.New].
func WithHint(err error, hint Hint) error {
	return rpcerror.WithDetails(err, &pulumirpc.HintDetail{Hint: hint.rpc()})
}

// HintRecorder is told about each hint that a provider reports for a failure during an operation.
type HintRecorder interface {
	RecordHint(pkg tokens.Package, hint Hint)
}

type hintRecorderKey struct{}

// WithHintRecorder returns a context whose provider calls report the hints for their failures to r.
func WithHintRecorder(ctx context.Context, r HintRecorder) context.Context {
	return context.WithValue(ctx, hintRecorderKey{}, r)
}

// recordHint reports hint for pkg to the recorder in ctx, if there is one and hint is specified.
func recordHint(ctx context.Context, pkg tokens.Package, hint Hint) {
	if hint == HintUnspecified || ctx == nil {
		return
	}
	if r, ok := ctx.Value(hintRecorderKey{}).(HintRecorder); ok {
		r.RecordHint(pkg, hint)
	}
}

// recordHint reports hint for pkg to the recorder of the call's ctx or, since the engine often calls providers
// with a detached context, to the recorder of the plugin context the provider was loaded with.
func (p *provider) recordHint(ctx context.Context, pkg tokens.Package, hint Hint) {
	if ctx != nil {
		if _, ok := ctx.Value(hintRecorderKey{}).(HintRecorder); ok {
			recordHint(ctx, pkg, hint)
			return
		}
	}
	if p.ctx != nil {
		recordHint(p.ctx.Base(), pkg, hint)
	}
}

// recordResourceHint reports the hint attached to err, if any, for the package of the resource urn.
func (p *provider) recordResourceHint(ctx context.Context, urn resource.URN, err error) {
	hint := HintOf(err)
	if hint == HintUnspecified {
		return
	}
	// Avoid tokens.Type.Package, which asserts that the token is well formed.
	pkg, _, _ := strings.Cut(string(urn.Type()), ":")
	p.recordHint(ctx, tokens.Package(pkg), hint)
}

// recordProviderHint reports hint, if specified, for the package of the provider type typ.
func (p *provider) recordProviderHint(ctx context.Context, typ tokens.Type, hint Hint) {
	if hint == HintUnspecified || !providers.IsProviderType(typ) {
		return
	}
	p.recordHint(ctx, providers.GetProviderPackage(typ), hint)
}
