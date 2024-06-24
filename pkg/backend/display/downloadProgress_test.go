// Copyright 2016-2024, Pulumi Corporation.
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

package display

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/stretchr/testify/assert"
)

func TestDownloadProgress(t *testing.T) {
	t.Parallel()

	t.Run("Size formatting", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			value    int64
			expected string
		}{
			{value: 0, expected: "0 B"},
			{value: 1, expected: "1 B"},
			{value: 1023, expected: "1023 B"},
			{value: 1024, expected: "1.00 KiB"},
			{value: 1024 * 1024, expected: "1.00 MiB"},
			{value: 1024 * 1024 * 1024, expected: "1.00 GiB"},
			{value: 808866, expected: "789.91 KiB"},
			{value: 164344, expected: "160.49 KiB"},
			{value: 174193, expected: "170.11 KiB"},
			{value: 696525823, expected: "664.26 MiB"},
			{value: 443626481, expected: "423.08 MiB"},
			{value: 186137911, expected: "177.51 MiB"},
		}
		for _, testcase := range cases {
			assert.Equalf(t, testcase.expected, formatBytes(testcase.value),
				"%d should render as `%s`", testcase.value, testcase.expected)
		}
	})

	t.Run("bar formatting", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			received, total int64
			width           int
			expected        string
		}{
			{received: 0, total: 100, width: 20, expected: "[                  ]"},
			{received: 50, total: 100, width: 20, expected: "[█████████         ]"},
			{received: 100, total: 100, width: 20, expected: "[██████████████████]"},
			{received: -1, total: 100, width: 20, expected: "[                  ]"},
			{received: 101, total: 100, width: 20, expected: "[██████████████████]"},
			{received: 40, total: 80, width: 12, expected: "[█████     ]"},
			{received: 41, total: 80, width: 12, expected: "[█████▏    ]"},
			{received: 42, total: 80, width: 12, expected: "[█████▎    ]"},
			{received: 43, total: 80, width: 12, expected: "[█████▍    ]"},
			{received: 44, total: 80, width: 12, expected: "[█████▌    ]"},
			{received: 45, total: 80, width: 12, expected: "[█████▋    ]"},
			{received: 46, total: 80, width: 12, expected: "[█████▊    ]"},
			{received: 47, total: 80, width: 12, expected: "[█████▉    ]"},
			{received: 48, total: 80, width: 12, expected: "[██████    ]"},
		}

		for _, testcase := range cases {
			assert.Equalf(t, testcase.expected, renderBar(testcase.received, testcase.total, testcase.width),
				"r: %d, t: %d, w:%d", testcase.received, testcase.total, testcase.width)
		}
	})

	t.Run("ASCII bar formatting", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			received, total int64
			width           int
			expected        string
		}{
			{received: 0, total: 100, width: 20, expected: "[__________________]"},
			{received: 50, total: 100, width: 20, expected: "[--------->________]"},
			{received: 100, total: 100, width: 20, expected: "[------------------]"},
			{received: -1, total: 100, width: 20, expected: "[__________________]"},
			{received: 101, total: 100, width: 20, expected: "[------------------]"},
			{received: 40, total: 80, width: 12, expected: "[----->____]"},
			{received: 41, total: 80, width: 12, expected: "[----->____]"},
			{received: 42, total: 80, width: 12, expected: "[----->____]"},
			{received: 43, total: 80, width: 12, expected: "[----->____]"},
			{received: 44, total: 80, width: 12, expected: "[----->____]"},
			{received: 45, total: 80, width: 12, expected: "[----->____]"},
			{received: 46, total: 80, width: 12, expected: "[----->____]"},
			{received: 47, total: 80, width: 12, expected: "[----->____]"},
			{received: 48, total: 80, width: 12, expected: "[------>___]"},
		}

		for _, testcase := range cases {
			assert.Equalf(t, testcase.expected, renderBarASCII(testcase.received, testcase.total, testcase.width),
				"r: %d, t: %d, w:%d", testcase.received, testcase.total, testcase.width)
		}
	})

	t.Run("Render download progress", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			received int64
			total    int64
			msg      string
			width    int
			expected string
		}{
			// simple case
			{
				received: 20, total: 100, msg: "Downloading plugin", width: 40,
				expected: "Downloading plugin [█▍     ]  20 B/100 B",
			},
			// Not enough room for numbers
			{
				received: 20, total: 100, msg: "Downloading plugin", width: 30,
				expected: "Downloading plugin [█▊       ]",
			},
			// Not enough room for bar
			{
				received: 20, total: 100, msg: "Downloading plugin", width: 20,
				expected: "Downloading plugin",
			},
			// Not enough room for entire message
			{
				received: 20, total: 100, msg: "Downloading plugin", width: 15,
				expected: "Downloading plu",
			},
		}

		for _, testcase := range cases {
			payload := engine.DownloadProgressEventPayload{
				Received:     testcase.received,
				Total:        testcase.total,
				Msg:          testcase.msg,
				DownloadType: engine.PluginDownload,
				ID:           "id",
			}
			assert.Equal(t, renderDownloadProgress(payload, testcase.width, false), testcase.expected)
		}
	})

	t.Run("ASCII Render download progress", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			received int64
			total    int64
			msg      string
			width    int
			expected string
		}{
			// simple case
			{
				received: 20, total: 100, msg: "Downloading plugin", width: 40,
				expected: "Downloading plugin [->_____]  20 B/100 B",
			},
			// Not enough room for numbers
			{
				received: 20, total: 100, msg: "Downloading plugin", width: 30,
				expected: "Downloading plugin [->_______]",
			},
			// Not enough room for bar
			{
				received: 20, total: 100, msg: "Downloading plugin", width: 20,
				expected: "Downloading plugin",
			},
			// Not enough room for entire message
			{
				received: 20, total: 100, msg: "Downloading plugin", width: 15,
				expected: "Downloading plu",
			},
		}

		for _, testcase := range cases {
			payload := engine.DownloadProgressEventPayload{
				Received:     testcase.received,
				Total:        testcase.total,
				Msg:          testcase.msg,
				DownloadType: engine.PluginDownload,
				ID:           "id",
			}
			assert.Equal(t, renderDownloadProgress(payload, testcase.width, true), testcase.expected)
		}
	})
}
