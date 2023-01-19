// Copyright 2016-2022, Pulumi Corporation.
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

package nodejs

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLazyLoadsGeneration(t *testing.T) {
	t.Parallel()

	ll := newLazyLoadGen()

	t.Run("resource", func(t *testing.T) { //nolint:paralleltest
		var buf bytes.Buffer
		ll.genReexport(&buf, fileInfo{
			fileType: resourceFileType,
			resourceFileInfo: resourceFileInfo{
				resourceClassName:         "MyRes",
				resourceArgsInterfaceName: "MyResArgs",
			},
		}, "./myResource")

		assert.Equal(t, `export { MyResArgs } from "./myResource";
export type MyRes = import("./myResource").MyRes;
export const MyRes: typeof import("./myResource").MyRes = null as any;
utilities.lazyLoad(exports, ["MyRes"], () => require("./myResource"));

`,
			buf.String())
	})

	t.Run("resource-with-state", func(t *testing.T) { //nolint:paralleltest
		var buf bytes.Buffer
		ll.genReexport(&buf, fileInfo{
			fileType: resourceFileType,
			resourceFileInfo: resourceFileInfo{
				resourceClassName:         "MyRes1",
				resourceArgsInterfaceName: "MyRes1Args",
				stateInterfaceName:        "MyRes1State",
			},
		}, "./myResource1")

		assert.Equal(t, `export { MyRes1Args, MyRes1State } from "./myResource1";
export type MyRes1 = import("./myResource1").MyRes1;
export const MyRes1: typeof import("./myResource1").MyRes1 = null as any;
utilities.lazyLoad(exports, ["MyRes1"], () => require("./myResource1"));

`,
			buf.String())
	})

	t.Run("resource-with-methods", func(t *testing.T) { //nolint:paralleltest
		var buf bytes.Buffer
		ll.genReexport(&buf, fileInfo{
			fileType: resourceFileType,
			resourceFileInfo: resourceFileInfo{
				resourceClassName:         "MyRes2",
				resourceArgsInterfaceName: "MyRes2Args",
				methodsNamespaceName:      "MyRes2",
			},
		}, "./myResource2")

		assert.Equal(t, `export * from "./myResource2";
import { MyRes2 } from "./myResource2";

`, buf.String())
	})

	t.Run("function", func(t *testing.T) { //nolint:paralleltest
		var buf bytes.Buffer
		ll.genReexport(&buf, fileInfo{
			fileType: functionFileType,
			functionFileInfo: functionFileInfo{
				functionName:                "myFunc",
				functionArgsInterfaceName:   "MyFuncArgs",
				functionResultInterfaceName: "MyFuncResult",
			},
		}, "./myFunc")

		assert.Equal(t, `export { MyFuncArgs, MyFuncResult } from "./myFunc";
export const myFunc: typeof import("./myFunc").myFunc = null as any;
utilities.lazyLoad(exports, ["myFunc"], () => require("./myFunc"));

`, buf.String())
	})

	t.Run("function-with-output-version", func(t *testing.T) { //nolint:paralleltest
		var buf bytes.Buffer
		ll.genReexport(&buf, fileInfo{
			fileType: functionFileType,
			functionFileInfo: functionFileInfo{
				functionName:                           "myFunc1",
				functionArgsInterfaceName:              "MyFunc1Args",
				functionResultInterfaceName:            "MyFunc1Result",
				functionOutputVersionName:              "myFunc1Output",
				functionOutputVersionArgsInterfaceName: "MyFunc1OutputArgs",
			},
		}, "./myFunc1")

		assert.Equal(t, `export { MyFunc1Args, MyFunc1Result, MyFunc1OutputArgs } from "./myFunc1";
export const myFunc1: typeof import("./myFunc1").myFunc1 = null as any;
export const myFunc1Output: typeof import("./myFunc1").myFunc1Output = null as any;
utilities.lazyLoad(exports, ["myFunc1","myFunc1Output"], () => require("./myFunc1"));

`, buf.String())
	})

	t.Run("fallthrough-reexport", func(t *testing.T) { //nolint:paralleltest
		var buf bytes.Buffer
		ll.genReexport(&buf, fileInfo{
			fileType: otherFileType,
		}, "./myOtherFile")

		assert.Equal(t, `export * from "./myOtherFile";
`, buf.String())
	})
}
