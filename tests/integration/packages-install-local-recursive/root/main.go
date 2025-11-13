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

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pkga "github.com/pulumi/pulumi/tests/integration/packages-install-local-recursive/root/sdks/pkg-a/go/pkg-a"
	pkgb "github.com/pulumi/pulumi/tests/integration/packages-install-local-recursive/root/sdks/pkg-b/go/pkg-b"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Create a component from pkg-a (TypeScript provider)
		componentA, err := pkga.NewSimpleComponent(ctx, "componentA", &pkga.SimpleComponentArgs{
			Message: pulumi.String("Hello from pkg-a"),
		})
		if err != nil {
			return err
		}

		// Create a component from pkg-b (Python provider)
		componentB, err := pkgb.NewSimpleComponent(ctx, "componentB", &pkgb.SimpleComponentArgs{
			Message: pulumi.String("Hello from pkg-b"),
		})
		if err != nil {
			return err
		}

		// Export the messages from both components
		ctx.Export("messageA", componentA.Message)
		ctx.Export("messageB", componentB.Message)
		return nil
	})
}
