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

//go:build !all

package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type BucketComponentV2 struct {
	pulumi.ResourceState
}

func NewBucketComponentV2(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*BucketComponentV2, error) {
	component := &BucketComponentV2{}
	err := ctx.RegisterRemoteComponentResource("wibble:index:BucketComponentV2", name, nil, component, opts...)
	if err != nil {
		return nil, err
	}
	return component, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := NewBucketComponentV2(ctx, "main-bucket")
		return err
	})
}
