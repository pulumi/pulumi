// Copyright 2016-2022, Pulumi Corporation.
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

// Terminal detection utilities.
package main

import "golang.org/x/term"

type optimalPageSizeOpts struct {
	nopts          int
	terminalHeight int
}

// Computes how many options to display in a Terminal UI multi-select.
// Tries to auto-detect and take terminal height into account.
func optimalPageSize(opts optimalPageSizeOpts) int {
	pageSize := 15
	if opts.terminalHeight != 0 {
		pageSize = opts.terminalHeight
	} else if _, height, err := term.GetSize(0); err == nil {
		pageSize = height
	}
	if pageSize > opts.nopts {
		pageSize = opts.nopts
	}
	const buffer = 5
	if pageSize > buffer {
		pageSize = pageSize - buffer
	}
	return pageSize
}
