// Copyright 2016-2018, Pulumi Corporation.
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
	"context"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// ErrorSource is a singleton source that returns an error if it is iterated. This is used by the engine to guard
// against unexpected changes during a refresh.
var ErrorSource Source = &errorSource{}

// A errorSource errors when iterated.
type errorSource struct{}

func (src *errorSource) Close() error                { return nil }
func (src *errorSource) Project() tokens.PackageName { return "" }
func (src *errorSource) Info() interface{}           { return nil }

func (src *errorSource) Iterate(ctx context.Context, opts Options, providers ProviderSource) (SourceIterator, error) {
	return nil, errors.New("internal error: unexpected call to errorSource.Iterate")
}
