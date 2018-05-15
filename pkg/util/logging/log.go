// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package logging

import (
	"flag"
	"fmt"
	"strconv"

	"github.com/golang/glog"
)

type filter interface {
	Filter(s string) string
}

var LogToStderr = false // true if logging is being redirected to stderr.
var Verbose = 0         // >0 if verbose logging is enabled at a particular level.
var LogFlow = false     // true to flow logging settings to child processes.
var Filter filter = &nopFilter{}

type nopFilter struct {
}

func (filter nopFilter) Filter(s string) string {
	return s
}

func V(level glog.Level) glog.Verbose {
	return glog.V(level)
}

func Errorf(format string, args ...interface{}) {
	glog.Errorf("%s", Filter.Filter(fmt.Sprintf(format, args...)))
}

func Infof(format string, args ...interface{}) {
	glog.Infof("%s", Filter.Filter(fmt.Sprintf(format, args...)))
}

func Warningf(format string, args ...interface{}) {
	glog.Warningf("%s", Filter.Filter(fmt.Sprintf(format, args...)))
}

func Flush() {
	glog.Flush()
}

// InitLogging ensures the logging library has been initialized with the given settings.
func InitLogging(logToStderr bool, verbose int, logFlow bool) {
	// Remember the settings in case someone inquires.
	LogToStderr = logToStderr
	Verbose = verbose
	LogFlow = logFlow

	// Ensure the logging library has been initialized, including calling flag.Parse beforehand.  Unfortunately,
	// this is the only way to control the way logging runs.  That includes poking around at flags below.
	flag.Parse()
	if logToStderr {
		err := flag.Lookup("logtostderr").Value.Set("true")
		assertNoError(err)
	}
	if verbose > 0 {
		err := flag.Lookup("v").Value.Set(strconv.Itoa(verbose))
		assertNoError(err)
	}
}

func assertNoError(err error) {
	if err != nil {
		failfast(err.Error())
	}
}

func failfast(msg string) {
	panic(fmt.Sprintf("fatal: %v", msg))
}

func SetFilter(filter filter) {
	Filter = filter
}
