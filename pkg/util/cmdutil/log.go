// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmdutil

import (
	"flag"
	"strconv"
)

// InitLogging ensures the glog library has been initialized with the given settings.
func InitLogging(logToStderr bool, verbose int) {
	// Ensure the glog library has been initialized, including calling flag.Parse beforehand.  Unfortunately,
	// this is the only way to control the way glog runs.  That includes poking around at flags below.
	flag.Parse()
	if logToStderr {
		flag.Lookup("logtostderr").Value.Set("true")
	}
	if verbose > 0 {
		flag.Lookup("v").Value.Set(strconv.Itoa(verbose))
	}
}
