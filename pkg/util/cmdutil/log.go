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

	"github.com/pulumi/lumi/pkg/util/contract"
)

var LogToStderr = false // true if logging is being redirected to stderr.
var Verbose = 0         // >0 if verbose logging is enabled at a particular level.
var LogFlow = false     // true to flow logging settings to child processes.

// InitLogging ensures the glog library has been initialized with the given settings.
func InitLogging(logToStderr bool, verbose int, logFlow bool) {
	// Remember the settings in case someone inquires.
	LogToStderr = logToStderr
	Verbose = verbose
	LogFlow = logFlow

	// Ensure the glog library has been initialized, including calling flag.Parse beforehand.  Unfortunately,
	// this is the only way to control the way glog runs.  That includes poking around at flags below.
	flag.Parse()
	if logToStderr {
		err := flag.Lookup("logtostderr").Value.Set("true")
		if err != nil {
			contract.Assert(err != nil)
		}

	}
	if verbose > 0 {
		err := flag.Lookup("v").Value.Set(strconv.Itoa(verbose))
		if err != nil {
			contract.Assert(err != nil)
		}
	}
}
