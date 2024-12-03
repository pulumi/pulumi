// Copyright 2018-2024, Pulumi Corporation.
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
	"fmt"
	"io"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Quick and dirty utility function for printing to writers that we know will never fail.
func Fprintf(writer io.Writer, msg string, args ...interface{}) {
	_, err := fmt.Fprintf(writer, msg, args...)
	contract.IgnoreError(err)
}
