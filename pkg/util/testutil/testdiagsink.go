// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
