// Copyright 2016, Pulumi Corporation.
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
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/runner"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/tests"
)

func main() {
	ctx := context.Background()
	server, err := runner.Start(ctx, tests.LanguageTestdata, tests.LanguageTests)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	os.Stdout.WriteString(server.Address())
	os.Stdout.Close()

	err = server.Done()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
