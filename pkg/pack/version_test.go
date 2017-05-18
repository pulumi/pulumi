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

package pack

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionCheck(t *testing.T) {
	// Legal versions:
	assert.Nil(t, Version("1.0.6").Check())
	assert.Nil(t, Version("1.0.6-beta").Check())
	assert.Nil(t, Version("1.0.6-beta+1").Check())
	assert.Nil(t, Version("76abc1f").Check())
	assert.Nil(t, Version("83030685c3b8a3dbe96bd10ab055f029667a96b0").Check())

	// Illegal versions:
	// 	- Empty
	assert.NotNil(t, Version("").Check())
	//  - Random non-SHA1 hash chars
	assert.NotNil(t, Version("a").Check())
	assert.NotNil(t, Version("8f93eef78239").Check())
	assert.NotNil(t, Version("!@#abcd").Check())
	// 	- Semantic version ranges
	assert.NotNil(t, Version(">1.0.6").Check())
}

func TestVersionSpecCheck(t *testing.T) {
	// Legal versions:
	//	- Latest
	assert.Nil(t, VersionSpec(LatestVersion).Check())
	//	- Semantic versions
	assert.Nil(t, VersionSpec("1.0.6").Check())
	assert.Nil(t, VersionSpec("1.0.6-beta").Check())
	assert.Nil(t, VersionSpec("1.0.6-beta+1").Check())
	assert.Nil(t, VersionSpec("76abc1f").Check())
	assert.Nil(t, VersionSpec("83030685c3b8a3dbe96bd10ab055f029667a96b0").Check())
	// 	- Semantic version ranges
	assert.NotNil(t, Version(">1.0.6").Check())

	// Illegal versions:
	// 	- Empty
	assert.NotNil(t, Version("").Check())
	//  - Random non-SHA1 hash chars
	assert.NotNil(t, Version("a").Check())
	assert.NotNil(t, Version("8f93eef78239").Check())
	assert.NotNil(t, Version("!@#abcd").Check())
}
