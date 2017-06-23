// Copyright 2016-2017, Pulumi Corporation
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

package resource

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewUniqueHex(t *testing.T) {
	prefix := "prefix"
	randlen := 20
	maxlen := 100
	id := NewUniqueHex(prefix, maxlen, randlen)
	assert.Equal(t, len(prefix)+randlen*2, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}

func Test_NewUniqueHex_Maxlen(t *testing.T) {
	prefix := "prefix"
	randlen := 20
	maxlen := 20
	id := NewUniqueHex(prefix, maxlen, randlen)
	assert.Equal(t, maxlen, len(id))
	assert.Equal(t, true, strings.HasPrefix(id, prefix))
}
