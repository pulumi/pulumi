// Copyright 2024, Pulumi Corporation.
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

func TestProgressBars(t *testing.T) {
	t.Parallel()

	t.Run("Size formatting", func(t *testing.T) {
		t.Parallel()

		// Arrange.
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
		for _, c := range cases {
			// Act.
			actual := formatBytes(c.value)

			// Assert.
			assert.Equalf(t, c.expected, actual, "%d should render as `%s`", c.value, c.expected)
		}
	})

	t.Run("Unicode progress bar formatting", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		cases := []struct {
			completed int64
			total     int64
			width     int
			expected  string
		}{
			{completed: 0, total: 100, width: 20, expected: "[                  ]"},
			{completed: 50, total: 100, width: 20, expected: "[█████████         ]"},
			{completed: 100, total: 100, width: 20, expected: "[██████████████████]"},
			{completed: -1, total: 100, width: 20, expected: "[                  ]"},
			{completed: 101, total: 100, width: 20, expected: "[██████████████████]"},
			{completed: 40, total: 80, width: 12, expected: "[█████     ]"},
			{completed: 41, total: 80, width: 12, expected: "[█████▏    ]"},
			{completed: 42, total: 80, width: 12, expected: "[█████▎    ]"},
			{completed: 43, total: 80, width: 12, expected: "[█████▍    ]"},
			{completed: 44, total: 80, width: 12, expected: "[█████▌    ]"},
			{completed: 45, total: 80, width: 12, expected: "[█████▋    ]"},
			{completed: 46, total: 80, width: 12, expected: "[█████▊    ]"},
			{completed: 47, total: 80, width: 12, expected: "[█████▉    ]"},
			{completed: 48, total: 80, width: 12, expected: "[██████    ]"},
		}

		for _, c := range cases {
			// Act.
			actual := renderUnicodeProgressBar(c.width, c.completed, c.total)

			// Assert.
			assert.Equalf(t, c.expected, actual, "r: %d, t: %d, w:%d", c.completed, c.total, c.width)
		}
	})

	t.Run("ASCII progress bar formatting", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		cases := []struct {
			completed int64
			total     int64
			width     int
			expected  string
		}{
			{completed: 0, total: 100, width: 20, expected: "[__________________]"},
			{completed: 50, total: 100, width: 20, expected: "[--------->________]"},
			{completed: 100, total: 100, width: 20, expected: "[------------------]"},
			{completed: -1, total: 100, width: 20, expected: "[__________________]"},
			{completed: 101, total: 100, width: 20, expected: "[------------------]"},
			{completed: 40, total: 80, width: 12, expected: "[----->____]"},
			{completed: 41, total: 80, width: 12, expected: "[----->____]"},
			{completed: 42, total: 80, width: 12, expected: "[----->____]"},
			{completed: 43, total: 80, width: 12, expected: "[----->____]"},
			{completed: 44, total: 80, width: 12, expected: "[----->____]"},
			{completed: 45, total: 80, width: 12, expected: "[----->____]"},
			{completed: 46, total: 80, width: 12, expected: "[----->____]"},
			{completed: 47, total: 80, width: 12, expected: "[----->____]"},
			{completed: 48, total: 80, width: 12, expected: "[------>___]"},
		}

		for _, c := range cases {
			// Act.
			actual := renderASCIIProgressBar(c.width, c.completed, c.total)

			// Assert.
			assert.Equalf(t, c.expected, actual, "r: %d, t: %d, w:%d", c.completed, c.total, c.width)
		}
	})

	t.Run("Unicode progress rendering", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		cases := []struct {
			typ       engine.ProgressType
			completed int64
			total     int64
			message   string
			width     int
			expected  string
		}{
			// Room for everything.
			{
				typ:       engine.PluginDownload,
				completed: 20,
				total:     100,
				message:   "Downloading plugin",
				width:     40,
				expected:  "Downloading plugin [█▍     ]  20 B/100 B",
			},
			// Not enough room for completed/total sizes.
			{
				typ:       engine.PluginDownload,
				completed: 20,
				total:     100,
				message:   "Downloading plugin",
				width:     30,
				expected:  "Downloading plugin [█▊       ]",
			},
			// Not enough room for the progress bar.
			{
				typ:       engine.PluginDownload,
				completed: 20,
				total:     100,
				message:   "Downloading plugin",
				width:     20,
				expected:  "Downloading plugin",
			},
			// Not enough room for the entire message.
			{
				typ:       engine.PluginDownload,
				completed: 20,
				total:     100,
				message:   "Downloading plugin",
				width:     15,
				expected:  "Downloading plu",
			},
		}

		for _, c := range cases {
			payload := engine.ProgressEventPayload{
				Type:      c.typ,
				Completed: c.completed,
				Total:     c.total,
				Message:   c.message,
				ID:        "id",
			}

			// Act.
			actual := renderProgress(renderUnicodeProgressBar, c.width, payload)

			// Assert.
			assert.Equal(t, c.expected, actual)
		}
	})

	t.Run("ASCII progress rendering", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		cases := []struct {
			typ       engine.ProgressType
			completed int64
			total     int64
			message   string
			width     int
			expected  string
		}{
			// Room for everything.
			{
				typ:       engine.PluginDownload,
				completed: 20,
				total:     100,
				message:   "Downloading plugin",
				width:     40,
				expected:  "Downloading plugin [->_____]  20 B/100 B",
			},
			// Not enough room for completed/total sizes.
			{
				typ:       engine.PluginDownload,
				completed: 20,
				total:     100,
				message:   "Downloading plugin",
				width:     30,
				expected:  "Downloading plugin [->_______]",
			},
			// Not enough room for the progress bar.
			{
				typ:       engine.PluginDownload,
				completed: 20,
				total:     100,
				message:   "Downloading plugin",
				width:     20,
				expected:  "Downloading plugin",
			},
			// Not enough room for the entire message.
			{
				typ:       engine.PluginDownload,
				completed: 20,
				total:     100,
				message:   "Downloading plugin",
				width:     15,
				expected:  "Downloading plu",
			},
		}

		for _, c := range cases {
			payload := engine.ProgressEventPayload{
				Completed: c.completed,
				Total:     c.total,
				Message:   c.message,
				Type:      engine.PluginDownload,
				ID:        "id",
			}

			// Act.
			actual := renderProgress(renderASCIIProgressBar, c.width, payload)

			// Assert.
			assert.Equal(t, c.expected, actual)
		}
	})
}
