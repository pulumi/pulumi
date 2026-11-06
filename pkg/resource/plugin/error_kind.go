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
		// A kind added in a newer protocol version than this engine knows.
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

// ErrorKindOf returns the kind of failure that err describes, as given by an ErrorKindDetail that the provider attached
// to the error. It returns ErrorKindUnspecified for errors without such a detail.
func ErrorKindOf(err error) ErrorKind {
	if err == nil {
		return ErrorKindUnspecified
	}
	rpcErr, ok := rpcerror.FromError(err)
	if !ok {
		if rpcErr, ok = errors.AsType[*rpcerror.Error](err); !ok {
			return ErrorKindUnspecified
		}
	}
	for _, d := range rpcErr.Details() {
		if detail, ok := d.(*pulumirpc.ErrorKindDetail); ok {
			if kind := errorKindFromRPC(detail.GetKind()); kind != ErrorKindUnspecified {
				return kind
			}
		}
	}
	return ErrorKindUnspecified
}

// WithErrorKind attaches an ErrorKindDetail to err, which must be a gRPC status error such as one made by
// [rpcerror.New].
func WithErrorKind(err error, kind ErrorKind) error {
	return rpcerror.WithDetails(err, &pulumirpc.ErrorKindDetail{Kind: kind.rpc()})
}

// ErrorKindRecorder is told about each classified failure that a provider reports during an operation.
type ErrorKindRecorder interface {
	RecordErrorKind(pkg tokens.Package, kind ErrorKind)
}

type errorKindRecorderKey struct{}

// WithErrorKindRecorder returns a context whose provider calls report classified failures to r.
func WithErrorKindRecorder(ctx context.Context, r ErrorKindRecorder) context.Context {
	return context.WithValue(ctx, errorKindRecorderKey{}, r)
}

// recordErrorKind reports kind for pkg to the recorder in ctx, if there is one and kind is specified.
func recordErrorKind(ctx context.Context, pkg tokens.Package, kind ErrorKind) {
	if kind == ErrorKindUnspecified || ctx == nil {
		return
	}
	if r, ok := ctx.Value(errorKindRecorderKey{}).(ErrorKindRecorder); ok {
		r.RecordErrorKind(pkg, kind)
	}
}

// recordErrorKind reports kind for pkg to the recorder of the call's ctx or, since the engine often calls providers
// with a detached context, to the recorder of the plugin context the provider was loaded with.
func (p *provider) recordErrorKind(ctx context.Context, pkg tokens.Package, kind ErrorKind) {
	if ctx != nil {
		if _, ok := ctx.Value(errorKindRecorderKey{}).(ErrorKindRecorder); ok {
			recordErrorKind(ctx, pkg, kind)
			return
		}
	}
	if p.ctx != nil {
		recordErrorKind(p.ctx.Base(), pkg, kind)
	}
}

// recordResourceErrorKind reports the kind of err, if any, for the package of the resource urn.
func (p *provider) recordResourceErrorKind(ctx context.Context, urn resource.URN, err error) {
	kind := ErrorKindOf(err)
	if kind == ErrorKindUnspecified {
		return
	}
	// Avoid tokens.Type.Package, which asserts that the token is well formed.
	pkg, _, _ := strings.Cut(string(urn.Type()), ":")
	p.recordErrorKind(ctx, tokens.Package(pkg), kind)
}

// recordProviderErrorKind reports kind, if specified, for the package of the provider type typ.
func (p *provider) recordProviderErrorKind(ctx context.Context, typ tokens.Type, kind ErrorKind) {
	if kind == ErrorKindUnspecified || !providers.IsProviderType(typ) {
		return
	}
	p.recordErrorKind(ctx, providers.GetProviderPackage(typ), kind)
}
