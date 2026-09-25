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

package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseToken(t *testing.T) {
	t.Parallel()

	valid := map[string]Token{
		"aws:s3/bucket:Bucket":      NewToken("aws", "s3/bucket", "Bucket"),
		"aws::Provider":             NewToken("aws", "", "Provider"),
		"my-pkg:index:getThing-abc": NewToken("my-pkg", "index", "getThing-abc"),
		"pkg:mod:_name.v1":          NewToken("pkg", "mod", "_name.v1"),
	}
	for s, want := range valid {
		tok, err := ParseToken(s)
		require.NoError(t, err, s)
		assert.Equal(t, want, tok, s)
	}

	invalid := []string{
		"",
		"pkg",
		"pkg:name",
		"pkg:mod:name:extra",
		"1pkg:mod:name",
		"pkg:1mod:name",
		"pkg:mod:1name",
		"pkg:mod:",
		"p kg:mod:name",
		"pkg:mod:na me",
	}
	for _, s := range invalid {
		_, err := ParseToken(s)
		assert.Error(t, err, s)
	}
}

func TestTokenText(t *testing.T) {
	t.Parallel()

	var tok Token
	require.NoError(t, tok.UnmarshalText([]byte("aws::Bucket")))
	assert.Equal(t, NewToken("aws", "", "Bucket"), tok)

	b, err := tok.AppendText([]byte("prefix:"))
	require.NoError(t, err)
	assert.Equal(t, "prefix:aws::Bucket", string(b))

	assert.Error(t, tok.UnmarshalText([]byte("bad")))
}

func TestTokenCmp(t *testing.T) {
	t.Parallel()

	a, b := NewToken("aws", "index", "Bucket"), NewToken("aws", "s3", "Bucket")
	assert.Equal(t, -1, a.Cmp(b))
	assert.Equal(t, 1, b.Cmp(a))
	assert.Equal(t, 0, a.Cmp(a))
	assert.True(t, a.Less(b))
	assert.False(t, b.Less(a))
	assert.False(t, a.Less(a))
}

func TestTokenZero(t *testing.T) {
	t.Parallel()

	var zero Token
	assert.True(t, zero.IsZero())
	assert.False(t, NewToken("aws", "", "Bucket").IsZero())
	assert.Equal(t, "", zero.String())

	var tok Token
	require.NoError(t, tok.UnmarshalText(nil))
	assert.True(t, tok.IsZero())

	_, err := ParseToken("")
	assert.Error(t, err)
}
