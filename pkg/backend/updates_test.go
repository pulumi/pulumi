// Copyright 2026, Pulumi Corporation.
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

package backend

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecordStackEnvironment(t *testing.T) {
	t.Parallel()

	t.Run("replaces the anonymous definition with the published revision", func(t *testing.T) {
		t.Parallel()

		m := &UpdateMetadata{Environment: map[string]string{}}
		SetStackEnvironmentsMetadata(m.Environment, []string{"shared/secrets", "yaml"})

		m.RecordStackEnvironment(StackConfiguration{
			EnvironmentImports: []string{"shared/secrets", "yaml"},
			StackEnvironment:   &StackEnvironmentRef{Name: "payments/prod", Revision: 7, OpenSessionID: "open-1"},
		})

		assert.Equal(t, "payments/prod@7", m.Environment[StackEnvironment])
		assert.Equal(t, "open-1", m.Environment[StackEnvironmentOpenSession])
		assert.JSONEq(t, `[{"id":"shared/secrets"},{"id":"payments/prod@7"}]`, m.Environment[StackEnvironments])
	})

	t.Run("appends the revision when the imports carry no anonymous entry", func(t *testing.T) {
		t.Parallel()

		m := &UpdateMetadata{}
		m.RecordStackEnvironment(StackConfiguration{
			StackEnvironment: &StackEnvironmentRef{Name: "payments/prod", Revision: 1},
		})

		assert.Equal(t, "payments/prod@1", m.Environment[StackEnvironment])
		assert.NotContains(t, m.Environment, StackEnvironmentOpenSession)
		assert.JSONEq(t, `[{"id":"payments/prod@1"}]`, m.Environment[StackEnvironments])
	})

	t.Run("is a no-op without a stack environment", func(t *testing.T) {
		t.Parallel()

		m := &UpdateMetadata{Environment: map[string]string{"stack.environments": `[{"id":"yaml"}]`}}
		m.RecordStackEnvironment(StackConfiguration{EnvironmentImports: []string{"yaml"}})
		assert.Equal(t, map[string]string{"stack.environments": `[{"id":"yaml"}]`}, m.Environment)

		var nilMetadata *UpdateMetadata
		nilMetadata.RecordStackEnvironment(StackConfiguration{StackEnvironment: &StackEnvironmentRef{Name: "a/b"}})
	})
}
