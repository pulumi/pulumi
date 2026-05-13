// Copyright 2026, Pulumi Corporation.
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

package neo

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// upgradeRequiredMessage is shown when any of the Neo API endpoints returns HTTP 426.
const upgradeRequiredMessage = "Your Pulumi CLI is out of date and is incompatible with the Pulumi Cloud API.\n" +
	"Please upgrade to the latest version: https://www.pulumi.com/docs/install/"

// upgradeWarnOnce ensures we only render the upgrade warning once per process — neo
// can hit 426 on multiple endpoints (CreateNeoTask, the SSE stream, follow-up
// tool_result posts) and we want a single, calm signal to the user.
var upgradeWarnOnce sync.Once

// resetUpgradeWarnOnceForTest is used by unit tests to reset the once between cases.
func resetUpgradeWarnOnceForTest() {
	upgradeWarnOnce = sync.Once{}
}

// isUpgradeRequired reports whether err signals an HTTP 426 from one of the neo
// API endpoints. The three neo endpoints all return *apitype.ErrorResponse for
// HTTP errors (CreateNeoTask/PostNeoTaskUserEvent via the shared pulumiAPICall
// path, StreamNeoTaskEvents via an explicit ErrorResponse return), so a single
// type check covers them.
func isUpgradeRequired(err error) bool {
	if err == nil {
		return false
	}
	var errResp *apitype.ErrorResponse
	return errors.As(err, &errResp) && errResp.Code == http.StatusUpgradeRequired
}

// warnUpgradeRequired reports whether err is a 426 from a neo endpoint and, if so,
// renders the upgrade warning exactly once. uiCh, when non-nil, receives a
// UIWarning; otherwise the warning is written to stderr. Returns true on a match
// so callers can substitute nil for the error and exit the command cleanly.
func warnUpgradeRequired(err error, uiCh chan<- UIEvent) bool {
	if !isUpgradeRequired(err) {
		return false
	}
	upgradeWarnOnce.Do(func() {
		if uiCh != nil {
			sendUI(uiCh, UIWarning{Message: upgradeRequiredMessage})
			return
		}
		fmt.Fprintln(os.Stderr, "warning: "+upgradeRequiredMessage)
	})
	return true
}
