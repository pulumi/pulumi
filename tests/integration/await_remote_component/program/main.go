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

//go:build !all

package main

import (
	"os"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type component struct {
	pulumi.ResourceState

	Out pulumi.StringOutput `pulumi:"out"`
}

type componentArgs struct {
	ReadyFile pulumi.StringInput `pulumi:"readyFile"`
}

func (componentArgs) ElementType() reflect.Type { return reflect.TypeFor[componentArgs]() }

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		var comp component
		if err := ctx.RegisterRemoteComponentResource("awaitcomp:index:Component", "comp",
			&componentArgs{ReadyFile: pulumi.String(os.Getenv("AWAIT_READY_FILE"))}, &comp); err != nil {
			return err
		}
		comp.Out.ApplyT(func(v string) string {
			// Record that the apply ran and with which value.
			_ = os.WriteFile(os.Getenv("AWAIT_APPLY_MARKER"), []byte(v), 0o600)
			return v
		})
		return nil
	})
}
