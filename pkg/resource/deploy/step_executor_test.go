// Copyright 2016-2023, Pulumi Corporation.
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
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestRegisterResourceErrorsOnMissingPendingNew(t *testing.T) {
	t.Parallel()

	se := &stepExecutor{
		pendingNews: sync.Map{},
	}
	urn := resource.URN("urn:pulumi:stack::project::my:example:Foo::foo")
	err := se.ExecuteRegisterResourceOutputs(&mockRegisterResourceOutputsEvent{
		urn: urn,
	})
	// Should error, but not panic since the resource is being registered twice.
	assert.Error(t, err)
}

type mockRegisterResourceOutputsEvent struct {
	urn resource.URN
}

var _ = RegisterResourceOutputsEvent((*mockRegisterResourceOutputsEvent)(nil))

func (e *mockRegisterResourceOutputsEvent) event() {}

func (e *mockRegisterResourceOutputsEvent) URN() resource.URN { return e.urn }

func (e *mockRegisterResourceOutputsEvent) Outputs() resource.PropertyMap {
	return resource.PropertyMap{}
}

func (e *mockRegisterResourceOutputsEvent) Done() {}
