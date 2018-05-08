// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package testutil

import (
	"io/ioutil"

	"github.com/pulumi/pulumi/pkg/diag"
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
		sink: diag.DefaultSink(ioutil.Discard, ioutil.Discard, diag.FormatOptions{
			Pwd: pwd,
		}),
		messages: make(map[diag.Severity][]string),
	}
}

func (d *TestDiagSink) DebugMsgs() []string   { return d.messages[diag.Debug] }
func (d *TestDiagSink) InfoMsgs() []string    { return d.messages[diag.Info] }
func (d *TestDiagSink) ErrorMsgs() []string   { return d.messages[diag.Error] }
func (d *TestDiagSink) WarningMsgs() []string { return d.messages[diag.Warning] }

func (d *TestDiagSink) Logf(sev diag.Severity, dia *diag.Diag, args ...interface{}) {
	d.messages[sev] = append(d.messages[sev], d.combine(sev, dia, args...))
}

func (d *TestDiagSink) Debugf(dia *diag.Diag, args ...interface{}) {
	d.messages[diag.Debug] = append(d.messages[diag.Debug], d.combine(diag.Debug, dia, args...))
}

func (d *TestDiagSink) Infof(dia *diag.Diag, args ...interface{}) {
	d.messages[diag.Info] = append(d.messages[diag.Info], d.combine(diag.Info, dia, args...))
}

func (d *TestDiagSink) Errorf(dia *diag.Diag, args ...interface{}) {
	d.messages[diag.Error] = append(d.messages[diag.Error], d.combine(diag.Error, dia, args...))
}

func (d *TestDiagSink) Warningf(dia *diag.Diag, args ...interface{}) {
	d.messages[diag.Warning] = append(d.messages[diag.Warning], d.combine(diag.Warning, dia, args...))
}

func (d *TestDiagSink) Stringify(sev diag.Severity, dia *diag.Diag, args ...interface{}) (string, string) {
	return d.sink.Stringify(sev, dia, args...)
}

func (d *TestDiagSink) combine(sev diag.Severity, dia *diag.Diag, args ...interface{}) string {
	p, s := d.sink.Stringify(sev, dia, args...)
	return p + s
}
