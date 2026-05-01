// Copyright 2016, Pulumi Corporation.
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

// Wrapper around slog that allows us to intercept all logging calls and manipulate them as
// necessary.  This is primarily used so we can make a best effort approach to filtering out secrets
// from any logs we emit before they get written to log-files/stderr.
//
// Code in pulumi may use this package instead of directly importing slog itself.  If any slog
// methods are needed that are not exported from this, they can be added, with the caveat that they
// should be updated to properly filter as well before forwarding things along.

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
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
	rwLock  sync.RWMutex
	filters []Filter
)

var (
	handlerMu   sync.RWMutex
	primary     slog.Handler = discardHandler{} // regular log output (stderr / file)
	sinkHandler slog.Handler                    // encrypted log handler, nil when inactive
	slogHandler *slog.Logger
	logFilePath string
	logFile     *os.File
)

func init() {
	rebuildLogger()

	// Register the standard logging flags on flag.CommandLine, matching
	// the behavior glog had via its own init(). Plugin binaries (language
	// hosts) use the standard flag package and receive these flags from
	// the CLI when --logflow is enabled.
	//
	// Guard with Lookup so we don't panic if a transitive dependency
	// (e.g. glog) has already registered the same flag name.
	if flag.CommandLine.Lookup("logtostderr") == nil {
		flag.BoolVar(&LogToStderr, "logtostderr", false,
			"Log to stderr instead of to files")
	}
	if flag.CommandLine.Lookup("v") == nil {
		flag.IntVar(&Verbose, "v", 0,
			"Enable verbose logging (e.g., v=3); anything >3 is very verbose")
	}
}

// rebuildLogger assembles the slog.Logger from the current primary and
// sink handlers. Must be called with handlerMu held for writing.
func rebuildLogger() {
	var h slog.Handler = formattingHandler{inner: filteringHandler{inner: primary}}
	if sinkHandler != nil {
		h = &teeHandler{primary: h, sink: sinkHandler}
	}
	slogHandler = slog.New(h)
}

// SetSinkHandler installs an additional slog.Handler that receives a
// copy of every log record.  Pass nil to remove the sink.  The handler
// must be safe for concurrent use.
func SetSinkHandler(h slog.Handler) {
	handlerMu.Lock()
	defer handlerMu.Unlock()
	sinkHandler = h
	rebuildLogger()
}

// sinkEnabled reports whether the sink handler exists and accepts
// records at the given level.
func sinkEnabled(level slog.Level) bool {
	handlerMu.RLock()
	defer handlerMu.RUnlock()
	return sinkHandler != nil && sinkHandler.Enabled(context.TODO(), level)
}

const LevelTrace = slog.LevelDebug - 4

// VerboseLogger logs messages only if verbosity matches the level it was built with.
type VerboseLogger struct{ level int32 }

// Enabled returns true if either slog verbose logging or the sink handler
// wants messages at this level.
func (v VerboseLogger) Enabled() bool {
	return v.slogEnabled() || sinkEnabled(v.slogLevel())
}

func (v VerboseLogger) slogEnabled() bool {
	return Verbose >= int(v.level) && v.level > 0
}

// slogLevel maps the pulumi verbosity level to a slog level:
//
//	V(1)–V(9)  → Info
//	V(10)      → Debug
//	V(11)+     → Trace
func (v VerboseLogger) slogLevel() slog.Level {
	switch {
	case v.level >= 11:
		return LevelTrace
	case v.level >= 10:
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

func (v VerboseLogger) Info(args ...any) {
	if v.Enabled() {
		slogHandler.Log(context.TODO(), v.slogLevel(), fmt.Sprint(args...), "v", int(v.level))
	}
}

// Infoln is equivalent to the global Infoln function, guarded by the value of v.
func (v VerboseLogger) Infoln(args ...any) {
	v.Info(args...)
}

// Infof is equivalent to the global Infof function, guarded by the value of v.
// The format string is stored as the slog message and each argument is
// recorded as a separate "pulumi.log.argN" attribute so that the sink
// handler can access them individually.
func (v VerboseLogger) Infof(format string, args ...any) {
	if v.Enabled() {
		slogHandler.Log(context.TODO(), v.slogLevel(), format, fmtAttrs(args, "v", int(v.level))...)
	}
}

func V(level int32) VerboseLogger {
	return VerboseLogger{level: level}
}

func Errorf(format string, args ...any) {
	slogHandler.Log(context.TODO(), slog.LevelError, format, fmtAttrs(args)...)
}

func Infof(format string, args ...any) {
	slogHandler.Log(context.TODO(), slog.LevelInfo, format, fmtAttrs(args)...)
}

func Warningf(format string, args ...any) {
	slogHandler.Log(context.TODO(), slog.LevelWarn, format, fmtAttrs(args)...)
}

// fmtAttrs encodes format arguments as slog key-value pairs so that
// downstream handlers can access each value individually.
// Extra key-value pairs (e.g. "v", level) are appended.
func fmtAttrs(args []any, extra ...any) []any {
	out := make([]any, 0, len(args)*2+len(extra))
	for i, a := range args {
		out = append(out, fmt.Sprintf("pulumi.log.arg%d", i), a)
	}
	return append(out, extra...)
}

func InitLogging(logToStderr bool, verbose int, logFlow bool) {
	LogToStderr = logToStderr
	Verbose = verbose
	LogFlow = logFlow

	// Parse flags so that CLI-provided values (e.g. -v, -logtostderr passed
	// via --logflow) are available. Then let non-default function arguments
	// win, matching the original glog-era behavior where InitLogging(false,0,false)
	// preserved whatever the flags said.
	if !flag.Parsed() {
		if err := flag.CommandLine.Parse([]string{}); err != nil {
			panic(fmt.Sprintf("failed to parse flags: %v", err))
		}
	}
	if logToStderr {
		LogToStderr = true
	} else if f := flag.CommandLine.Lookup("logtostderr"); f != nil {
		LogToStderr = f.Value.String() == "true"
	}
	if verbose > 0 {
		Verbose = verbose
	} else if f := flag.CommandLine.Lookup("v"); f != nil {
		fmt.Sscan(f.Value.String(), &Verbose) //nolint:errcheck
	}

	handlerMu.Lock()
	defer handlerMu.Unlock()

	if LogToStderr {
		primary = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: LevelTrace,
		})
	} else if Verbose > 0 {
		f, err := os.Create(logFileName())
		if err == nil {
			logFilePath = f.Name()
			logFile = f
			primary = slog.NewJSONHandler(f, &slog.HandlerOptions{
				Level: LevelTrace,
			})
		}
	}
	rebuildLogger()
}

// logFileName returns a log file path matching the glog naming convention:
// <program>.<host>.<user>.log.<severity>.<YYYYMMDD>-<HHMMSS>.<pid>
func logFileName() string {
	program := filepath.Base(os.Args[0])
	host, _ := os.Hostname()
	if i := strings.IndexByte(host, '.'); i >= 0 {
		host = host[:i]
	}
	username := "unknownuser"
	if u, err := user.Current(); err == nil {
		username = u.Username
		// On Windows, Username is often DOMAIN\user. Replace path separators
		// so the log filename doesn't accidentally create subdirectories.
		username = strings.ReplaceAll(username, string(filepath.Separator), "_")
	}
	now := time.Now()
	name := fmt.Sprintf("%s.%s.%s.log.INFO.%04d%02d%02d-%02d%02d%02d.%d",
		program, host, username, now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second(), os.Getpid())
	return filepath.Join(os.TempDir(), name)
}

// Flush flushes any pending log I/O.
func Flush() {
	if logFile != nil {
		logFile.Sync() //nolint:errcheck
	}
}

func GetLogfilePath() (string, error) {
	if logFilePath != "" {
		return logFilePath, nil
	}
	return "", errors.New("no log files found")
}

// teeHandler fans out slog records to two handlers.
type teeHandler struct {
	primary slog.Handler
	sink    slog.Handler
}

func (t *teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return t.primary.Enabled(ctx, level) || t.sink.Enabled(ctx, level)
}

func (t *teeHandler) Handle(ctx context.Context, r slog.Record) error {
	if t.primary.Enabled(ctx, r.Level) {
		_ = t.primary.Handle(ctx, r)
	}
	if t.sink.Enabled(ctx, r.Level) {
		_ = t.sink.Handle(ctx, r)
	}
	return nil
}

func (t *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	p := t.primary.WithAttrs(attrs)
	s := t.sink.WithAttrs(attrs)
	return &teeHandler{primary: p, sink: s}
}

func (t *teeHandler) WithGroup(name string) slog.Handler {
	return &teeHandler{primary: t.primary.WithGroup(name), sink: t.sink.WithGroup(name)}
}

// formattingHandler reconstructs the formatted message from
// pulumi.log.argN attributes before forwarding to its inner handler.
// This lets the primary log output show the fully formatted message
// while the sink handler can access each argument individually.
type formattingHandler struct {
	inner slog.Handler
}

func (f formattingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return f.inner.Enabled(ctx, level)
}

func (f formattingHandler) Handle(ctx context.Context, r slog.Record) error {
	// Collect pulumi.log.argN values (in order) and all other attrs.
	var fmtArgs []any
	var other []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		if strings.HasPrefix(a.Key, "pulumi.log.arg") {
			fmtArgs = append(fmtArgs, a.Value.Any())
		} else {
			other = append(other, a)
		}
		return true
	})

	msg := r.Message
	if len(fmtArgs) > 0 {
		msg = fmt.Sprintf(r.Message, fmtArgs...)
	}

	newRec := slog.NewRecord(r.Time, r.Level, msg, r.PC)
	newRec.AddAttrs(other...)
	return f.inner.Handle(ctx, newRec)
}

func (f formattingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return formattingHandler{inner: f.inner.WithAttrs(attrs)}
}

func (f formattingHandler) WithGroup(name string) slog.Handler {
	return formattingHandler{inner: f.inner.WithGroup(name)}
}

type nopFilter struct{}

func (f *nopFilter) Filter(s string) string {
	return s
}

type replacerFilter struct {
	replacer *strings.Replacer
}

func (f *replacerFilter) Filter(s string) string {
	return f.replacer.Replace(s)
}

func AddGlobalFilter(filter Filter) {
	rwLock.Lock()
	filters = append(filters, filter)
	rwLock.Unlock()
}

func CreateFilter(secrets []string, replacement string) Filter {
	items := slice.Prealloc[string](len(secrets))
	for _, secret := range secrets {
		// For short secrets, don't actually add them to the filter, this is a trade-off we make to prevent
		// displaying `[secret]`. Travis does a similar thing, for example.
		if len(secret) < 3 {
			continue
		}
		if strings.EqualFold(secret, "true") || strings.EqualFold(secret, "false") {
			continue
		}
		items = append(items, secret, replacement)

		// Catch secrets that are serialized to JSON.
		bs, err := json.Marshal(secret)
		if err != nil {
			continue
		}
		if escaped := string(bs[1 : len(bs)-1]); escaped != secret {
			items = append(items, escaped, replacement)
		}
	}
	if len(items) > 0 {
		return &replacerFilter{replacer: strings.NewReplacer(items...)}
	}

	return &nopFilter{}
}

func FilterString(msg string) string {
	var localFilters []Filter
	rwLock.RLock()
	localFilters = filters
	rwLock.RUnlock()

	for _, filter := range localFilters {
		msg = filter.Filter(msg)
	}

	return msg
}

type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (d discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return d }
func (d discardHandler) WithGroup(string) slog.Handler           { return d }

// filteringHandler wraps any slog.Handler and applies FilterString
// to the record's message and string-typed attributes before forwarding.
type filteringHandler struct {
	inner slog.Handler
}

func (f filteringHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return f.inner.Enabled(ctx, level)
}

func (f filteringHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Message = FilterString(r.Message)
	newRec := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		newRec.AddAttrs(filterAttr(a))
		return true
	})
	return f.inner.Handle(ctx, newRec)
}

func (f filteringHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	filtered := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		filtered[i] = filterAttr(a)
	}
	return filteringHandler{inner: f.inner.WithAttrs(filtered)}
}

func (f filteringHandler) WithGroup(name string) slog.Handler {
	return filteringHandler{inner: f.inner.WithGroup(name)}
}

func filterAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindString {
		a.Value = slog.StringValue(FilterString(a.Value.String()))
	}
	return a
}
