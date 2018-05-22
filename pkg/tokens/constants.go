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

package tokens

// Accessibility determines the visibility of a class member.
type Accessibility string

// Accessibility modifiers.
const (
	PublicAccessibility    Accessibility = "public"
	PrivateAccessibility   Accessibility = "private"
	ProtectedAccessibility Accessibility = "protected"
)

// Special module names.
const (
	DefaultModule ModuleName = ".default" // used to reference the default module.
)

// Special variable names.
const (
	ThisVariable  Name = ".this"  // the current object (for class methods).
	SuperVariable Name = ".super" // the parent class object (for class methods).
)

// Special function names.
const (
	EntryPointFunction       ModuleMemberName = ".main" // the special package entrypoint function.
	ModuleInitFunction       ModuleMemberName = ".init" // the special module initialization function.
	ClassConstructorFunction ClassMemberName  = ".ctor" // the special class instance constructor function.
	ClassInitFunction        ClassMemberName  = ".init" // the special class initialization function.
)
