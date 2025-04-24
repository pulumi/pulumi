// Copyright 2016-2023, Pulumi Corporation.
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

package deploytest

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
)

type NoopSink struct {
	LogfF func(sev diag.Severity, diag *diag.Diag, args ...interface{})
}

var _ diag.Sink = (*NoopSink)(nil)

func (s *NoopSink) Logf(sev diag.Severity, diag *diag.Diag, args ...interface{}) {
	if s.LogfF != nil {
		s.LogfF(sev, diag, args)
	}
}

func (s *NoopSink) Debugf(diag *diag.Diag, args ...interface{}) {}

func (s *NoopSink) Infof(diag *diag.Diag, args ...interface{}) {}

func (s *NoopSink) Infoerrf(diag *diag.Diag, args ...interface{}) {}

func (s *NoopSink) Errorf(diag *diag.Diag, args ...interface{}) {}

func (s *NoopSink) Warningf(diag *diag.Diag, args ...interface{}) {}

func (s *NoopSink) Stringify(
	sev diag.Severity, diag *diag.Diag, args ...interface{},
) (string, string) {
	return "", ""
}
