// Copyright 2016-2020, Pulumi Corporation.
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

package main

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestParseLocation(t *testing.T) {
	tests := []struct {
		pipShowOutput string
		expected      string
		err           error
	}{
		{
			pipShowOutput: "Location: /plugin/location",
			expected:      "/plugin/location",
		},
		{
			pipShowOutput: "Location:/plugin/location",
			expected:      "/plugin/location",
		},
		{
			pipShowOutput: "Location:   /plugin/location",
			expected:      "/plugin/location",
		},
		{
			pipShowOutput: "Foo: bar\nLocation: /plugin/location\n",
			expected:      "/plugin/location",
		},
		{
			pipShowOutput: "Foo: bar\nLocation: /plugin/location\nBlah: baz",
			expected:      "/plugin/location",
		},
		{
			pipShowOutput: "Foo: bar\r\nLocation: /plugin/location\r\nBlah: baz",
			expected:      "/plugin/location",
		},
		{
			pipShowOutput: "",
			err:           errors.New("determining location of package foo"),
		},
		{
			pipShowOutput: "Foo: bar\nBlah: baz",
			err:           errors.New("determining location of package foo"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.pipShowOutput, func(t *testing.T) {
			result, err := parseLocation("foo", tt.pipShowOutput)
			if tt.err != nil {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.err.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeterminePluginVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		err      error
	}{
		{
			input:    "0.1",
			expected: "0.1",
		},
		{
			input:    "1.0",
			expected: "1.0",
		},
		{
			input:    "1.0.0",
			expected: "1.0.0",
		},
		{
			input: "",
			err:   errors.New(`unexpected number of components in version ""`),
		},
		{
			input: "2",
			err:   errors.New(`unexpected number of components in version "2"`),
		},
		{
			input: "4.3.2.1",
			err:   errors.New(`unexpected number of components in version "4.3.2.1"`),
		},
		{
			input: " 1 . 2 . 3 ",
			err:   errors.New(`parsing major: " 1 "`),
		},
		{
			input: "2.1a123456789",
			err:   errors.New(`parsing minor: "1a123456789"`),
		},
		{
			input: "2.14.0a1605583329",
			err:   errors.New(`parsing patch: "0a1605583329"`),
		},
		{
			input: "1.2.3b123456",
			err:   errors.New(`parsing patch: "3b123456"`),
		},
		{
			input: "3.2.1rc654321",
			err:   errors.New(`parsing patch: "1rc654321"`),
		},
		{
			input: "1.2.3dev7890",
			err:   errors.New(`parsing patch: "3dev7890"`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := determinePluginVersion(tt.input)
			if tt.err != nil {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.err.Error())
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
