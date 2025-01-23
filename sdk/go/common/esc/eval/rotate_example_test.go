// Copyright 2025, Pulumi Corporation.
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

package eval

import (
	"context"
	"fmt"
	"github.com/pulumi/esc"
)

func ExampleRotate() {
	const def = `
values:
  a:
    a:
      fn::rotate:
        provider: swap
        inputs:
          foo: bar
        state:
          a: a
          b: b
    b:
    - c:
        fn::rotate::swap:
          inputs:
            foo: bar
          state:
            a: 
              fn::secret: a
            b: b
`
	env, _, _ := LoadYAMLBytes("<stdin>", []byte(def))

	// rotate the environment
	execContext, _ := esc.NewExecContext(nil)
	_, patches, _ := RotateEnvironment(context.Background(), "<stdin>", env, rot128{}, testProviders{}, &testEnvironments{}, execContext)

	// writeback state patches
	updated, _ := ApplyValuePatches([]byte(def), patches)

	// encrypt secret values
	encryptedYaml, _ := EncryptSecrets(context.Background(), "<stdin>", updated, rot128{})

	fmt.Println(string(encryptedYaml))
	// Output:
	// values:
	//   a:
	//     a:
	//       fn::rotate:
	//         provider: swap
	//         inputs:
	//           foo: bar
	//         state:
	//           a: b
	//           b: a
	//     b:
	//       - c:
	//           fn::rotate::swap:
	//             inputs:
	//               foo: bar
	//             state:
	//               a: b
	//               b:
	//                 fn::secret:
	//                   ciphertext: ZXNjeAAAAAHhQRt8TQ==
}
