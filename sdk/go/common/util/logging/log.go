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

package logging

// Wrapper around the glog API that allows us to intercept all logging calls and manipulate them as
// necessary.  This is primarily used so we can make a best effort approach to filtering out secrets
// from any logs we emit before they get written to log-files/stderr.
//
// Code in pulumi should use this package instead of directly importing glog itself.  If any glog
// methods are needed that are not exported from this, they can be added, with the caveat that they
// should be updated to properly filter as well before forwarding things along.

import (
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"sync"

	"github.com/golang/glog"
	aho_corasick "github.com/pgavlin/aho-corasick"
)

type Filter interface {
	Filter(s string) string
}

var (
	LogToStderr = false // true if logging is being redirected to stderr.
	Verbose     = 0     // >0 if verbose logging is enabled at a particular level.
	LogFlow     = false // true to flow logging settings to child processes.
)

var (
	rwLock       sync.RWMutex
	globalFilter *globalAhoCorasickFilter
	filters      []Filter
)

// VerboseLogger logs messages only if verbosity matches the level it was built with.
//
// It may be used as a boolean to check if it's enabled.
//
//	if log := logging.V(lvl); log {
//		log.Infoln(expensiveComputation())
//	}
type VerboseLogger glog.Verbose

// Info is equivalent to the global Info function, guarded by the value of v.
// See the documentation of V for usage.
func (v VerboseLogger) Info(args ...interface{}) {
	if v {
		glog.Verbose(v).Info(FilterString(fmt.Sprint(args...)))
	}
}

// Infoln is equivalent to the global Infoln function, guarded by the value of v.
// See the documentation of V for usage.
func (v VerboseLogger) Infoln(args ...interface{}) {
	if v {
		glog.Verbose(v).Infoln(FilterString(fmt.Sprint(args...)))
	}
}

// Infof is equivalent to the global Infof function, guarded by the value of v.
// See the documentation of V for usage.
func (v VerboseLogger) Infof(format string, args ...interface{}) {
	if v {
		glog.Verbose(v).Infof("%s", FilterString(fmt.Sprintf(format, args...)))
	}
}

// V builds a logger that logs messages only if verbosity is at least at the provided level.
func V(level glog.Level) VerboseLogger {
	return VerboseLogger(glog.V(level))
}

func Errorf(format string, args ...interface{}) {
	glog.Errorf("%s", FilterString(fmt.Sprintf(format, args...)))
}

func Infof(format string, args ...interface{}) {
	glog.Infof("%s", FilterString(fmt.Sprintf(format, args...)))
}

func Warningf(format string, args ...interface{}) {
	glog.Warningf("%s", FilterString(fmt.Sprintf(format, args...)))
}

func Flush() {
	glog.Flush()
}

func maybeSetFlag(name, value string) {
	if f := flag.Lookup(name); f != nil {
		err := f.Value.Set(value)
		assertNoError(err)
	}
}

// InitLogging ensures the logging library has been initialized with the given settings.
func InitLogging(logToStderr bool, verbose int, logFlow bool) {
	// Remember the settings in case someone inquires.
	LogToStderr = logToStderr
	Verbose = verbose
	LogFlow = logFlow

	// glog uses golang's built in flags package to set configuration values, which is incompatible with how
	// we use cobra. In order to accommodate this, we call flag.CommandLine.Parse() with an empty array and
	// explicitly set the flags we care about here.
	if !flag.Parsed() {
		err := flag.CommandLine.Parse([]string{})
		assertNoError(err)
	}
	if logToStderr {
		maybeSetFlag("logtostderr", "true")
	}
	if verbose > 0 {
		maybeSetFlag("v", strconv.Itoa(verbose))
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

type nopFilter struct{}

func (f *nopFilter) Filter(s string) string {
	return s
}

type ahoCorasickFilter struct {
	replacement string
	replacer    aho_corasick.Replacer
}

func (f *ahoCorasickFilter) Filter(s string) string {
	return f.replacer.ReplaceAllWith(s, f.replacement)
}

type globalAhoCorasickFilter struct {
	patterns     []string
	replacements []string
	replacer     aho_corasick.Replacer
}

func (f *globalAhoCorasickFilter) filter(s string) string {
	return f.replacer.ReplaceAll(s, f.replacements)
}

func AddGlobalFilter(filter Filter) {
	rwLock.Lock()
	filters = append(filters, filter)
	rwLock.Unlock()
}

func CreateGlobalFilter(secrets []string, replacement string) {
	rwLock.Lock()
	defer rwLock.Unlock()

	patterns := makePatterns(secrets)
	if len(patterns) == 0 {
		return
	}
	replacements := make([]string, len(patterns))
	for i := range replacements {
		replacements[i] = replacement
	}

	if globalFilter != nil {
		patterns = append(patterns, globalFilter.patterns...)
		replacements = append(replacements, globalFilter.replacements...)
	}

	builder := aho_corasick.NewAhoCorasickBuilder(aho_corasick.Opts{
		MatchKind: aho_corasick.StandardMatch,
	})
	replacer := aho_corasick.NewReplacer(builder.Build(patterns))

	globalFilter = &globalAhoCorasickFilter{
		patterns:     patterns,
		replacements: replacements,
		replacer:     replacer,
	}
}

func makePatterns(secrets []string) []string {
	patterns := make([]string, 0, len(secrets)*2)
	for _, secret := range secrets {
		// For short secrets, don't actually add them to the filter, this is a trade-off we make to prevent
		// displaying `[secret]`. Travis does a similar thing, for example.
		if len(secret) < 3 {
			continue
		}
		patterns = append(patterns, secret)

		// Catch secrets that are serialized to JSON.
		bs, err := json.Marshal(secret)
		if err != nil {
			continue
		}
		if escaped := string(bs[1 : len(bs)-1]); escaped != secret {
			patterns = append(patterns, escaped)
		}
	}
	return patterns
}

func CreateFilter(secrets []string, replacement string) Filter {
	patterns := makePatterns(secrets)
	if len(patterns) == 0 {
		return &nopFilter{}
	}

	builder := aho_corasick.NewAhoCorasickBuilder(aho_corasick.Opts{
		MatchKind: aho_corasick.StandardMatch,
	})
	return &ahoCorasickFilter{
		replacement: replacement,
		replacer:    aho_corasick.NewReplacer(builder.Build(patterns)),
	}
}

func FilterString(msg string) string {
	var localGlobalFilter *globalAhoCorasickFilter
	var localFilters []Filter
	rwLock.RLock()
	localGlobalFilter = globalFilter
	localFilters = filters
	rwLock.RUnlock()

	if localGlobalFilter != nil {
		msg = localGlobalFilter.filter(msg)
	}
	for _, filter := range localFilters {
		msg = filter.Filter(msg)
	}

	return msg
}
