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

package testutil

import (
	"github.com/pulumi/lumi/pkg/diag"
)

// TestDiagSink suppresses message output, but captures them, so that they can be compared to expected results.
type TestDiagSink struct {
	Pwd      string
	sink     diag.Sink
	messages map[diag.Severity][]string
}

func NewTestDiagSink(pwd string) *TestDiagSink {
	return &TestDiagSink{
		Pwd: pwd,
		sink: diag.DefaultSink(diag.FormatOptions{
			Pwd: pwd,
		}),
		messages: make(map[diag.Severity][]string),
	}
}

func (d *TestDiagSink) Count() int            { return d.Debugs() + d.Infos() + d.Errors() + d.Warnings() }
func (d *TestDiagSink) Debugs() int           { return len(d.DebugMsgs()) }
func (d *TestDiagSink) DebugMsgs() []string   { return d.messages[diag.Debug] }
func (d *TestDiagSink) Infos() int            { return len(d.InfoMsgs()) }
func (d *TestDiagSink) InfoMsgs() []string    { return d.messages[diag.Info] }
func (d *TestDiagSink) Errors() int           { return len(d.ErrorMsgs()) }
func (d *TestDiagSink) ErrorMsgs() []string   { return d.messages[diag.Error] }
func (d *TestDiagSink) Warnings() int         { return len(d.WarningMsgs()) }
func (d *TestDiagSink) WarningMsgs() []string { return d.messages[diag.Warning] }
func (d *TestDiagSink) Success() bool         { return d.Errors() == 0 }

func (d *TestDiagSink) Logf(sev diag.Severity, dia *diag.Diag, args ...interface{}) {
	d.messages[sev] = append(d.messages[sev], d.Stringify(sev, dia, args...))
}

func (d *TestDiagSink) Debugf(dia *diag.Diag, args ...interface{}) {
	d.messages[diag.Debug] = append(d.messages[diag.Debug], d.Stringify(diag.Debug, dia, args...))
}

func (d *TestDiagSink) Infof(dia *diag.Diag, args ...interface{}) {
	d.messages[diag.Info] = append(d.messages[diag.Info], d.Stringify(diag.Info, dia, args...))
}

func (d *TestDiagSink) Errorf(dia *diag.Diag, args ...interface{}) {
	d.messages[diag.Error] = append(d.messages[diag.Error], d.Stringify(diag.Error, dia, args...))
}

func (d *TestDiagSink) Warningf(dia *diag.Diag, args ...interface{}) {
	d.messages[diag.Warning] = append(d.messages[diag.Warning], d.Stringify(diag.Warning, dia, args...))
}

func (d *TestDiagSink) Stringify(sev diag.Severity, dia *diag.Diag, args ...interface{}) string {
	return d.sink.Stringify(sev, dia, args...)
}

func (d *TestDiagSink) StringifyLocation(sev diag.Severity, doc *diag.Document, loc *diag.Location) string {
	return d.sink.StringifyLocation(sev, doc, loc)
}
