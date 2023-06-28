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

package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitLogging(t *testing.T) {
	t.Parallel()

	// Just ensure we can initialize logging (and reset it afterwards).
	prevLog := LogToStderr
	prevV := Verbose
	prevFlow := LogFlow
	InitLogging(true, 9, true)
	InitLogging(prevLog, prevV, prevFlow)
	assert.Equal(t, prevLog, LogToStderr)
	assert.Equal(t, prevV, Verbose)
	assert.Equal(t, prevFlow, LogFlow)
}

func TestFilter(t *testing.T) {
	t.Parallel()

	filter1 := CreateFilter([]string{"secret1", "secret2"}, "[secret]")
	msg1 := filter1.Filter(
		"These are my secrets: secret1, secret2, secret3, secret10")
	assert.Equal(t,
		"These are my secrets: [secret], [secret], secret3, [secret]0",
		msg1)

	// Ensure that special characters don't screw up the search
	filter2 := CreateFilter([]string{"secret.*", "secre[t]3"}, "[creds]")
	msg2 := filter2.Filter(
		"These are my secrets: secret1, secret2, secret3, secret.*, secre[t]3")
	assert.Equal(t,
		"These are my secrets: secret1, secret2, secret3, [creds], [creds]",
		msg2)

	// Ensure that non-UTF8 characters don't screw up the search
	filter3 := CreateFilter([]string{"nonutf8\xa7", "secret1"}, "[creds]")
	msg3 := filter3.Filter(
		"These are my secrets: secret1, nonutf8\xa7")
	assert.Equal(t,
		"These are my secrets: [creds], [creds]",
		msg3)

	// Short secrets of 1-2 characters are not masked
	filter4 := CreateFilter([]string{"a", "my", "123"}, "[creds]")
	msg4 := filter4.Filter(
		"These are my secrets: a, my, 123")
	assert.Equal(t,
		"These are my secrets: a, my, [creds]",
		msg4)

	// Ensure that multi-line secrets are masked in output.
	filter5 := CreateFilter([]string{"multi\nline\nsecret"}, "[secret]")
	msg5 := filter5.Filter(
		`These are my secrets: multi\nline\nsecret`)
	assert.Equal(t,
		"These are my secrets: [secret]",
		msg5)

	// Ensure that secrets with tabs are masked in output.
	filter6 := CreateFilter([]string{"secretwith\t"}, "[secret]")
	msg6 := filter6.Filter(
		`These are my secrets: secretwith\t`)
	assert.Equal(t,
		"These are my secrets: [secret]",
		msg6)
}

func TestGlobalFilter(t *testing.T) {
	t.Parallel()

	CreateGlobalFilter([]string{"secret1", "secret2"}, "[secret]")
	msg1 := FilterString("These are my secrets: secret1, secret2, secret3, secret10")
	assert.Equal(t, "These are my secrets: [secret], [secret], secret3, [secret]0", msg1)

	CreateGlobalFilter([]string{"creds1", "creds2"}, "[credentials]")
	msg2 := FilterString("These are my secrets: secret1, secret2, secret3, secret10, creds1, creds2, creds")
	assert.Equal(t,
		"These are my secrets: [secret], [secret], secret3, [secret]0, [credentials], [credentials], creds", msg2)
}
