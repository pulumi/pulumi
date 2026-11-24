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

package cli

import (
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
)

// newTable returns a table.Writer configured with the esc CLI's standard style.
// Callers are expected to append a header, append rows, and then Render().
func newTable(stdout io.Writer) table.Writer {
	t := table.NewWriter()
	t.SetOutputMirror(stdout)
	t.SetStyle(table.StyleLight)
	return t
}
