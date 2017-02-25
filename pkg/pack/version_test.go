// Copyright 2016 Pulumi, Inc. All rights reserved.

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
