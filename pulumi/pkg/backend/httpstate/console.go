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

package httpstate

import (
	"net/url"
	"path"
	"strings"
)

func cloudConsoleURL(cloudURL string, paths ...string) string {
	u, err := url.Parse(cloudURL)
	if err != nil {
		return ""
	}

	switch {
	case strings.HasPrefix(u.Host, defaultAPIDomainPrefix):
		u.Host = defaultConsoleDomainPrefix + u.Host[len(defaultAPIDomainPrefix):]
	case u.Host == "localhost:8080":
		u.Host = "localhost:3000"
	default:
		return "" // We couldn't figure out how to convert the api hostname into a console hostname
	}

	u.Path = path.Join(paths...)
	return u.String()
}
