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

//go:build darwin

package securestore

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// CoreFoundation bindings via purego (no cgo); CFTypeRefs are carried as
// uintptr. CF Create Rule: refs from Create/Copy functions must be released
// exactly once; Get-style refs are borrowed and must not be.

const (
	coreFoundationPath = "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
	securityPath       = "/System/Library/Frameworks/Security.framework/Security"

	// kCFStringEncodingUTF8 from CoreFoundation/CFString.h.
	kCFStringEncodingUTF8 = 0x08000100
	// kCFNumberSInt64Type from CoreFoundation/CFNumber.h.
	kCFNumberSInt64Type = 4
)

// Accumulates the first symbol-resolution error, so call sites can bind many
// symbols without per-symbol error handling.
type lib struct {
	path   string
	handle uintptr
	err    error
}

func openLib(path string) *lib {
	l := &lib{path: path}
	handle, err := purego.Dlopen(path, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		l.err = fmt.Errorf("loading %s: %w", path, err)
		return l
	}
	l.handle = handle
	return l
}

func (l *lib) fn(fptr any, name string) {
	if l.err != nil {
		return
	}
	sym, err := purego.Dlsym(l.handle, name)
	if err != nil {
		l.err = fmt.Errorf("resolving %s in %s: %w", name, l.path, err)
		return
	}
	purego.RegisterFunc(fptr, sym)
}

// Binds a symbol that may not exist, leaving the variable nil. For the
// deprecated SecKeychain family: its removal should disable only the features
// needing it, not every Security binding. Callers must nil-check.
func (l *lib) optionalFn(fptr any, name string) {
	if l.err != nil {
		return
	}
	if sym, err := purego.Dlsym(l.handle, name); err == nil {
		purego.RegisterFunc(fptr, sym)
	}
}

// For struct-typed symbols (e.g. kCFTypeDictionaryKeyCallBacks) that C passes
// by address.
func (l *lib) addr(name string) uintptr {
	if l.err != nil {
		return 0
	}
	sym, err := purego.Dlsym(l.handle, name)
	if err != nil {
		l.err = fmt.Errorf("resolving %s in %s: %w", name, l.path, err)
		return 0
	}
	return sym
}

// For pointer-typed symbols like `const CFStringRef kSecClass`: Dlsym yields
// the variable's address, so one dereference gives the value.
func (l *lib) constant(name string) uintptr {
	addr := l.addr(name)
	if addr == 0 {
		return 0
	}
	// Double-indirect through &addr so go vet does not flag a plain
	// uintptr-to-unsafe.Pointer conversion; addr points at immortal dyld data.
	return **(**uintptr)(unsafe.Pointer(&addr))
}

type cfAPI struct {
	release            func(ref uintptr)
	stringCreate       func(alloc uintptr, cstr string, encoding uint32) uintptr
	stringGetLength    func(s uintptr) int
	stringGetCString   func(s uintptr, buf []byte, size int, encoding uint32) bool
	stringGetTypeID    func() uintptr
	dataCreate         func(alloc uintptr, bytes []byte, length int) uintptr
	dataGetLength      func(data uintptr) int
	dataGetBytePtr     func(data uintptr) *byte
	dictionaryCreate   func(alloc uintptr, keys, values *uintptr, count int, keyCB, valueCB uintptr) uintptr
	dictionaryGetValue func(dict, key uintptr) uintptr
	numberGetValue     func(num uintptr, numType int, out *int64) bool
	getTypeID          func(ref uintptr) uintptr

	typeDictKeyCallBacks   uintptr
	typeDictValueCallBacks uintptr
	booleanTrue            uintptr
}

func newCFAPI(l *lib) *cfAPI {
	c := &cfAPI{}
	l.fn(&c.release, "CFRelease")
	l.fn(&c.stringCreate, "CFStringCreateWithCString")
	l.fn(&c.stringGetLength, "CFStringGetLength")
	l.fn(&c.stringGetCString, "CFStringGetCString")
	l.fn(&c.stringGetTypeID, "CFStringGetTypeID")
	l.fn(&c.dataCreate, "CFDataCreate")
	l.fn(&c.dataGetLength, "CFDataGetLength")
	l.fn(&c.dataGetBytePtr, "CFDataGetBytePtr")
	l.fn(&c.dictionaryCreate, "CFDictionaryCreate")
	l.fn(&c.dictionaryGetValue, "CFDictionaryGetValue")
	l.fn(&c.numberGetValue, "CFNumberGetValue")
	l.fn(&c.getTypeID, "CFGetTypeID")
	c.typeDictKeyCallBacks = l.addr("kCFTypeDictionaryKeyCallBacks")
	c.typeDictValueCallBacks = l.addr("kCFTypeDictionaryValueCallBacks")
	c.booleanTrue = l.constant("kCFBooleanTrue")
	return c
}

// Caller releases.
func (c *cfAPI) newString(s string) uintptr {
	return c.stringCreate(0, s, kCFStringEncodingUTF8)
}

// Borrowed ref.
func (c *cfAPI) goString(s uintptr) string {
	n := c.stringGetLength(s) // length in UTF-16 code units
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n*4+1) // worst-case UTF-8 expansion plus NUL
	if !c.stringGetCString(s, buf, len(buf), kCFStringEncodingUTF8) {
		return ""
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

// Caller releases.
func (c *cfAPI) newData(b []byte) uintptr {
	return c.dataCreate(0, b, len(b))
}

func (c *cfAPI) dataBytes(data uintptr) []byte {
	n := c.dataGetLength(data)
	if n <= 0 {
		return nil
	}
	ptr := c.dataGetBytePtr(data)
	if ptr == nil {
		return nil
	}
	out := make([]byte, n)
	copy(out, unsafe.Slice(ptr, n))
	return out
}

// Caller releases. Retains its keys and values, so caller-created refs may be
// released immediately after.
func (c *cfAPI) newDict(keys, values []uintptr) uintptr {
	return c.dictionaryCreate(0, &keys[0], &values[0], len(keys),
		c.typeDictKeyCallBacks, c.typeDictValueCallBacks)
}

var (
	darwinAPIOnce sync.Once
	darwinAPIErr  error
	// cf and sec are non-nil iff loadDarwinAPI returned nil.
	cf  *cfAPI
	sec *secAPI
)

// Failure is an error, not a panic, so probing can fall back.
func loadDarwinAPI() error {
	darwinAPIOnce.Do(func() {
		cfLib := openLib(coreFoundationPath)
		secLib := openLib(securityPath)
		cfBound := newCFAPI(cfLib)
		secBound := newSecAPI(secLib)
		if cfLib.err != nil {
			darwinAPIErr = cfLib.err
			return
		}
		if secLib.err != nil {
			darwinAPIErr = secLib.err
			return
		}
		cf, sec = cfBound, secBound
	})
	return darwinAPIErr
}
