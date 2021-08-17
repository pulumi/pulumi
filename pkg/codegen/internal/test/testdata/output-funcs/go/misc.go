// Copyright 2016-2021, Pulumi Corporation.
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

// This code is hand-added to make generated code compile.
package codegentest

import (
	"context"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// An access key for the storage account.
type StorageAccountKeyResponse struct {
	// Name of the key.
	KeyName string `pulumi:"keyName"`
	// Permissions for the key -- read-only or full permissions.
	Permissions string `pulumi:"permissions"`
	// Base 64-encoded value of the key.
	Value string `pulumi:"value"`
}

type StorageAccountKeyResponseArrayOutput struct{ *pulumi.OutputState }

func (StorageAccountKeyResponseArrayOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*[]StorageAccountKeyResponse)(nil)).Elem()
}

func (o StorageAccountKeyResponseArrayOutput) ToStorageAccountKeyResponseArrayOutput() StorageAccountKeyResponseArrayOutput {
	return o
}

func (o StorageAccountKeyResponseArrayOutput) ToStorageAccountKeyResponseArrayOutputWithContext(ctx context.Context) StorageAccountKeyResponseArrayOutput {
	return o
}

func init() {
	pulumi.RegisterOutputType(StorageAccountKeyResponseArrayOutput{})
}
