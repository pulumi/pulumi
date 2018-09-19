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

package result

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Smoke test for All, composing multiple Results into a single Result.
func TestAll(t *testing.T) {
	assert.Nil(t, All([]*Result{nil, nil, nil}))

	bail := All([]*Result{nil, nil, Bail(), nil})
	assert.NotNil(t, bail)
	assert.Nil(t, bail.Error())

	errRes := All([]*Result{nil, nil, Error("oh no"), nil})
	assert.NotNil(t, errRes)
	assert.NotNil(t, errRes.Error())
}
