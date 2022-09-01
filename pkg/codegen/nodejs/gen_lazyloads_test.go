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
	ll := newLazyLoadGen()

	t.Run("resource", func(t *testing.T) {
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
utilities.lazyLoadProperty(exports, "MyRes", () => require("./myResource"));

`,
			buf.String())
	})

}
