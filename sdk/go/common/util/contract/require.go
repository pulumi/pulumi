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

package contract

import (
	"fmt"
)

const requireMsg = "A precondition has failed for %v"

// Require checks a precondition condition pertaining to a function parameter, and Fails if it is false.
func Require(cond bool, param string) {
	if !cond {
		failfast(fmt.Sprintf(requireMsg, param))
	}
}

// Requiref checks a precondition condition pertaining to a function parameter, and Failfs if it is false.
func Requiref(cond bool, param string, msg string, args ...interface{}) {
	if !cond {
		failfast(fmt.Sprintf("%v: %v", fmt.Sprintf(requireMsg, param), fmt.Sprintf(msg, args...)))
	}
}
