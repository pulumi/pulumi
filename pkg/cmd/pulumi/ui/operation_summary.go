// Copyright 2016-2025, Pulumi Corporation.
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

package ui

import (
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
)

// OperationSummaryJSON is the structured summary emitted after a stack operation completes.
type OperationSummaryJSON struct {
	Result        string            `json:"result"`
	Changes       ChangeSummaryJSON `json:"changes"`
	Duration      string            `json:"duration"`
	ResourceCount int               `json:"resourceCount"`
	Outputs       map[string]any    `json:"outputs,omitempty"`
}

// ChangeSummaryJSON contains counts for each type of resource change.
type ChangeSummaryJSON struct {
	Create  int `json:"create"`
	Update  int `json:"update"`
	Delete  int `json:"delete"`
	Same    int `json:"same"`
	Replace int `json:"replace"`
}

// NewChangeSummaryJSON converts ResourceChanges to ChangeSummaryJSON.
func NewChangeSummaryJSON(changes display.ResourceChanges) ChangeSummaryJSON {
	return ChangeSummaryJSON{
		Create:  changes[deploy.OpCreate],
		Update:  changes[deploy.OpUpdate],
		Delete:  changes[deploy.OpDelete],
		Same:    changes[deploy.OpSame],
		Replace: changes[deploy.OpReplace],
	}
}
