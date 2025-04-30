// Copyright 2025, Pulumi Corporation.
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
	"errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

type Contexts struct {
	ctxs map[key]*Context
}

type key struct {
	project string
	stack   string
}

func NewContexts(ctxs map[resource.AbsoluteStackReference]*Context) *Contexts {
	keyedCtxs := make(map[key]*Context, len(ctxs))
	for stackRef, ctx := range ctxs {
		if ctx == nil {
			continue
		}

		k := key{
			project: stackRef.Project,
			stack:   stackRef.Stack,
		}

		keyedCtxs[k] = ctx
	}

	return &Contexts{
		ctxs: keyedCtxs,
	}
}

func (h *Contexts) GetHostByURN(urn resource.URN) Host {
	return nil
}

func (h *Contexts) GetHostByStackReference(stackRef resource.AbsoluteStackReference) Host {
	return nil
}

func (h *Contexts) SignalCancellation() error {
	errs := make([]error, 0, len(h.ctxs))
	for _, ctx := range h.ctxs {
		if err := ctx.Host.SignalCancellation(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (h *Contexts) Close() error {
	errs := make([]error, 0, len(h.ctxs))
	for _, ctx := range h.ctxs {
		if err := ctx.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
