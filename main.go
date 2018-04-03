// Copyright 2017-2018, Pulumi Corporation.
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
	"fmt"
	"os"

	"github.com/pulumi/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func main() {
	if err := cmd.NewPulumiCmd().Execute(); err != nil {
		_, err = fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
		contract.IgnoreError(err)
		os.Exit(-1)
	}
}
