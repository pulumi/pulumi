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

package pulumi

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"golang.org/x/mod/modfile"
)

const pulumiSDKModule = "github.com/pulumi/pulumi/sdk/v3"

var stackTraceRuntimeSource = sync.OnceValue(func() string {
	function := runtime.FuncForPC(reflect.ValueOf(runtime.Callers).Pointer())
	file, _ := function.FileLine(function.Entry())
	return filepath.Dir(filepath.Dir(file))
})

var stackTraceBuildInfo = sync.OnceValue(func() *debug.BuildInfo {
	info, _ := debug.ReadBuildInfo()
	return info
})

type stackTraceModule struct {
	path string
	dir  string
}

var stackTraceMainModule = sync.OnceValue(func() stackTraceModule {
	info := stackTraceBuildInfo()
	if info == nil || info.Main.Path == "" {
		return stackTraceModule{}
	}
	wd, err := os.Getwd()
	if err != nil {
		return stackTraceModule{}
	}
	return findStackTraceModule(wd, info.Main.Path)
})

func findStackTraceModule(programDir, modulePath string) stackTraceModule {
	dir, err := fsutil.WalkUpDirs(programDir, func(dir string) bool {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		return err == nil && modfile.ModulePath(data) == modulePath
	})
	if err != nil {
		return stackTraceModule{}
	}
	return stackTraceModule{path: modulePath, dir: dir}
}

// sourceFile resolves a first-party -trimpath location against the Go module directory.
func (m stackTraceModule) sourceFile(file string) string {
	if m.dir == "" {
		return file
	}
	suffix, ok := strings.CutPrefix(file, m.path+"/")
	relative := filepath.FromSlash(suffix)
	if ok && filepath.IsLocal(relative) {
		return filepath.Join(m.dir, relative)
	}
	return file
}

var stackTraceModules = sync.OnceValue(func() []string {
	modules := []string{pulumiSDKModule}
	if info := stackTraceBuildInfo(); info != nil {
		if info.Main.Path != "" {
			modules = append(modules, info.Main.Path)
		}
		for _, dependency := range info.Deps {
			// Keep the module's import identity, including when its source is replaced locally.
			modules = append(modules, dependency.Path)
		}
	}
	slices.SortFunc(modules, func(a, b string) int { return len(b) - len(a) })
	return modules
})

func stackFramePackage(function string) string {
	// Receivers and generic arguments can contain dots and slashes of their own.
	if index := strings.IndexAny(function, "(["); index >= 0 {
		function = function[:index]
	}
	lastSlash := strings.LastIndexByte(function, '/')
	if dot := strings.IndexByte(function[lastSlash+1:], '.'); dot >= 0 {
		name := function[:lastSlash+1+dot]
		if decoded, err := url.PathUnescape(name); err == nil {
			return decoded
		}
		return name
	}
	return ""
}

func stackFrameIdentity(frame runtime.Frame, modules []string) (module, origin string) {
	pkg := stackFramePackage(frame.Function)
	if pkg == "" {
		return "", ""
	}
	if pkg == "main" {
		if info := stackTraceBuildInfo(); info != nil && info.Main.Path != "" {
			return info.Main.Path, ""
		}
	}
	for _, candidate := range modules {
		if pkg == candidate || strings.HasPrefix(pkg, candidate+"/") {
			if candidate == pulumiSDKModule {
				return candidate, "pulumi"
			}
			return candidate, ""
		}
	}
	// Standard-library frames have GOROOT source paths, or package-relative paths under -trimpath.
	runtimeSource := filepath.ToSlash(stackTraceRuntimeSource())
	file := filepath.ToSlash(frame.File)
	standardLibrary := filepath.IsAbs(runtimeSource) &&
		strings.HasPrefix(file, runtimeSource+"/")
	first, _, _ := strings.Cut(pkg, "/")
	trimmedStandardLibrary := pkg != "main" && !strings.Contains(first, ".") && strings.HasPrefix(file, pkg+"/")
	if standardLibrary || trimmedStandardLibrary {
		return pkg, "runtime"
	}
	// Test binaries and stripped build information may not identify the enclosing module.
	return pkg, ""
}

// getStackTrace captures its caller's stack, omitting skip additional caller frames.
// runtime.Callers and getStackTrace itself are always omitted.
func (ctx *Context) getStackTrace(skip int) *pulumirpc.StackTrace {
	pcs := make([]uintptr, 64)
	var count int
	for {
		count = runtime.Callers(skip+2, pcs)
		if count < len(pcs) {
			break
		}
		pcs = make([]uintptr, len(pcs)*2)
	}
	trace := &pulumirpc.StackTrace{}
	if count == 0 {
		return trace
	}
	frames := runtime.CallersFrames(pcs[:count])
	for {
		frame, more := frames.Next()
		module, origin := stackFrameIdentity(frame, stackTraceModules())
		trace.Frames = append(trace.Frames, &pulumirpc.StackFrame{
			Pc: ctx.getSourcePositionForFrame(frame), Module: module, Origin: origin,
		})
		if !more {
			break
		}
	}
	return trace
}
