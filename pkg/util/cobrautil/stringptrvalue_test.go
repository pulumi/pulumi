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

package cobrautil

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestStringPtrValue(t *testing.T) {
	t.Parallel()

	t.Run("flag with value", func(t *testing.T) {
		t.Parallel()
		var value *string
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		NewStringPtrVar(cmd.Flags(), &value, "test-flag", "test usage")

		err := cmd.ParseFlags([]string{"--test-flag=myvalue"})
		assert.NoError(t, err)
		assert.NotNil(t, value)
		assert.Equal(t, "myvalue", *value)
	})

	t.Run("flag without value (no opt)", func(t *testing.T) {
		t.Parallel()
		var value *string
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		NewStringPtrVar(cmd.Flags(), &value, "test-flag", "test usage")

		err := cmd.ParseFlags([]string{"--test-flag"})
		assert.NoError(t, err)
		assert.NotNil(t, value)
		assert.Equal(t, "", *value)
	})

	t.Run("flag not set", func(t *testing.T) {
		t.Parallel()
		var value *string
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		NewStringPtrVar(cmd.Flags(), &value, "test-flag", "test usage")

		err := cmd.ParseFlags([]string{})
		assert.NoError(t, err)
		assert.Nil(t, value)
	})

	t.Run("flag with empty string value", func(t *testing.T) {
		t.Parallel()
		var value *string
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		NewStringPtrVar(cmd.Flags(), &value, "test-flag", "test usage")

		err := cmd.ParseFlags([]string{"--test-flag="})
		assert.NoError(t, err)
		assert.NotNil(t, value)
		assert.Equal(t, "", *value)
	})

	t.Run("flag with whitespace value", func(t *testing.T) {
		t.Parallel()
		var value *string
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		NewStringPtrVar(cmd.Flags(), &value, "test-flag", "test usage")

		err := cmd.ParseFlags([]string{"--test-flag=  spaces  "})
		assert.NoError(t, err)
		assert.NotNil(t, value)
		assert.Equal(t, "  spaces  ", *value)
	})
}
