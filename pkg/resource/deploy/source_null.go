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

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// NullSource is a source that never returns any resources.  This may be used in scenarios where the "new"
// version of the world is meant to be empty, either for testing purposes, or removal of an existing stack.
func NewNullSource(project tokens.PackageName) Source {
	return &nullSource{project: project}
}

// A nullSource never returns any resources.
type nullSource struct {
	project tokens.PackageName
}

// Deprecated: A NullSource with no project name.
var NullSource Source = &nullSource{}

func (src *nullSource) Close() error                { return nil }
func (src *nullSource) Project() tokens.PackageName { return src.project }

func (src *nullSource) Iterate(ctx context.Context, providers ProviderSource) (SourceIterator, error) {
	contract.Ignore(ctx)
	return &nullSourceIterator{}, nil
}

// nullSourceIterator always returns nil, nil in response to Next, indicating that it is done.
type nullSourceIterator struct{}

func (iter *nullSourceIterator) Cancel(context.Context) error {
	return nil // nothing to do.
}

func (iter *nullSourceIterator) Next() (SourceEvent, error) {
	return nil, nil // means "done"
}
