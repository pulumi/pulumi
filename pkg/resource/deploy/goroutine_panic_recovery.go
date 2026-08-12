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

package deploy

import (
	"fmt"
	"runtime/debug"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// PanicRecovery wraps a goroutine function with panic recovery logic.
// If a panic occurs, it logs the panic and stack trace, then sends the error to the provided error channel.
// The function will not send to the error channel if panicErrs is nil.
func PanicRecovery(panicErrs chan<- error, fn func()) {
	if env.GoroutinePanicRecovery.Value() && panicErrs != nil {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				err := fmt.Errorf("panic in goroutine: %v\nStack trace:\n%s", r, string(stack))
				logging.V(3).Infof("Recovered from panic: %v", err)

				select {
				case panicErrs <- err:
				default:
					panic(r)
				}
			}
		}()
	}
	fn()
}
