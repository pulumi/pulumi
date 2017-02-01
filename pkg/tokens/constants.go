// Copyright 2016 Marapongo, Inc. All rights reserved.

package tokens

type Accessibility string // accessibility modifiers common to all.

// Module accessibility modifiers.
const (
	PublicAccessibility  Accessibility = "public"
	PrivateAccessibility Accessibility = "private"
)

type ClassMemberAccessibility Accessibility // accessibility modifiers for class members.

// Class member accessibility modifiers.
const (
	PublicClassAccessibility    ClassMemberAccessibility = "public"
	PrivateClassAccessibility   ClassMemberAccessibility = "private"
	ProtectedClassAccessibility ClassMemberAccessibility = "protected"
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
