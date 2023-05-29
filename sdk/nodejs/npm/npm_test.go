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

package npm

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// This test checks that os.exec call for `npm install` is well constructed.
func TestNPMInstallCmd(t *testing.T) {
	t.Parallel()
	cases := []struct {
		production   bool
		expectedArgs []string
	}{
		{
			production:   true,
			expectedArgs: []string{"false", "install", "--loglevel=error", "--production"},
		}, {
			production:   false,
			expectedArgs: []string{"false", "install", "--loglevel=error"},
		},
	}

	pkgManager := &npmManager{
		executable: "false", // a fake path for testing.
	}
	ctx := context.Background()

	for _, tc := range cases {
		tc := tc
		name := fmt.Sprintf("production=%v", tc.production)
		t.Run(name, func(tt *testing.T) {
			tt.Parallel()
			command := pkgManager.installCmd(ctx, tc.production)
			// Compare our expectations against observations.
			expected := tc.expectedArgs
			observed := command.Args
			assert.ElementsMatch(t, expected, observed)
			// Next, we check if the binary name matches our expectations.
			// Trim the absolute path, since it's system dependent.
			observedCommand := filepath.Base(command.Path)
			// Trim the extension, which will appear on Windows systems.
			if extension := filepath.Ext(observedCommand); extension != "" {
				observedCommand = strings.TrimSuffix(observedCommand, extension)
			}
			assert.Equal(t, "false", observedCommand)
		})
	}
}
