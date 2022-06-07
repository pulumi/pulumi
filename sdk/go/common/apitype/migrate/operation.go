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

package migrate

import "github.com/pulumi/pulumi/sdk/v3/go/common/apitype"

// UpToOperationV2 migrates a resource from OperationV1 to OperationV2.
func UpToOperationV2(v1 apitype.OperationV1) apitype.OperationV2 {
	return apitype.OperationV2{
		Resource: UpToResourceV3(v1.Resource),
		Type:     v1.Type,
	}
}
