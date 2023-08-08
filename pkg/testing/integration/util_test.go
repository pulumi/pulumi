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

package integration

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenLogFile(t *testing.T) {
	t.Parallel()

	runDir := t.TempDir()

	log1, err := openLogFile("foo", runDir)
	require.NoError(t, err)

	assert.Contains(t, log1.Name(), "foo",
		"log file name should contain the name of the command")
	assert.Contains(t, log1.Name(), commandOutputFolderName,
		"log file name should contain the name of the command output folder")

	log2, err := openLogFile("foo", runDir)
	require.NoError(t, err)

	assert.NotEqual(t, log1.Name(), log2.Name(),
		"log file names should be unique")

	_, err = io.WriteString(log1, "hello")
	require.NoError(t, err)

	_, err = io.WriteString(log2, "world")
	require.NoError(t, err)

	require.NoError(t, log1.Close())
	require.NoError(t, log2.Close())

	log1Body, err := os.ReadFile(log1.Name())
	require.NoError(t, err)

	log2Body, err := os.ReadFile(log2.Name())
	require.NoError(t, err)

	assert.Equal(t, "hello", string(log1Body))
	assert.Equal(t, "world", string(log2Body))
}
