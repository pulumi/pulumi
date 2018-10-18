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

package display

import "github.com/pulumi/pulumi/pkg/diag/colors"

// Options controls how the output of events are rendered
type Options struct {
	Color                colors.Colorization // colorization to apply to events.
	ShowConfig           bool                // true if we should show configuration information.
	ShowReplacementSteps bool                // true to show the replacement steps in the plan.
	ShowSameResources    bool                // true to show the resources that aren't updated in addition to updates.
	SuppressOutputs      bool                // true to suppress output summarization, e.g. if contains sensitive info.
	SummaryDiff          bool                // If the diff display should be summarized
	IsInteractive        bool                // If we should display things interactively
	DiffDisplay          bool                // true if we should display things as a rich diff
	Debug                bool                // true to enable debug output.
}
