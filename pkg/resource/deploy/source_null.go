// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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
	"github.com/pulumi/lumi/pkg/resource"
)

// NullSource is a singleton source that never returns any resources.  This may be used in scenarios where the "new"
// version of the world is meant to be empty, either for testing purposes, or removal of an existing environment.
var NullSource Source = &nullSource{}

// A nullSource never returns any resources.
type nullSource struct {
}

func (src *nullSource) Close() error {
	return nil // nothing to do.
}

func (src *nullSource) Info() interface{} {
	return nil
}

func (src *nullSource) Iterate() (SourceIterator, error) {
	return &nullSourceIterator{}, nil
}

// nullSourceIterator always returns nil, nil in response to Next, indicating that it is done.
type nullSourceIterator struct {
}

func (iter *nullSourceIterator) Close() error {
	return nil // nothing to do.
}

func (iter *nullSourceIterator) Produce(res *resource.Object) {
	// ignore
}

func (iter *nullSourceIterator) Next() (*SourceAllocation, *SourceQuery, error) {
	return nil, nil, nil // means "done"
}
