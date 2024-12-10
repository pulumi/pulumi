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

package cmd

import "golang.org/x/term"

type OptimalPageSizeOpts struct {
	Nopts          int
	TerminalHeight int
}

// Computes how many options to display in a Terminal UI multi-select.
// Tries to auto-detect and take terminal height into account.
func OptimalPageSize(opts OptimalPageSizeOpts) int {
	pageSize := 15
	if opts.TerminalHeight != 0 {
		pageSize = opts.TerminalHeight
	} else if _, height, err := term.GetSize(0); err == nil {
		pageSize = height
	}
	if pageSize > opts.Nopts {
		pageSize = opts.Nopts
	}
	const buffer = 5
	if pageSize > buffer {
		pageSize = pageSize - buffer
	}
	return pageSize
}
