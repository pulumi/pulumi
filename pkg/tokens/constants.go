// Copyright 2016 Marapongo, Inc. All rights reserved.

package tokens

// Module accessibility.
type Accessibility string // accessibility modifiers common to all.
const (
	PublicAccessibility  Accessibility = "public"
	PrivateAccessibility Accessibility = "private"
)

// Class member accessibility.
type ClassMemberAccessibility Accessibility // accessibility modifiers for class members.
const (
	PublicClassAccessibility    ClassMemberAccessibility = "public"
	PrivateClassAccessibility   ClassMemberAccessibility = "private"
	ProtectedClassAccessibility ClassMemberAccessibility = "protected"
)

// Special variable tokens.
const (
	ThisVariable  Name = ".this"  // the current object (for class methods).
	SuperVariable Name = ".super" // the parent class object (for class methods).
)

// Special function tokens.
const (
	EntryPointFunction        ModuleMemberName = ".main" // the special package entrypoint function.
	ModuleInitializerFunction ModuleMemberName = ".init" // the special module initialization function.
	ClassConstructorFunction  ClassMemberName  = ".ctor" // the special class instance constructor function.
	ClassInitializerFunction  ClassMemberName  = ".init" // the special class initialization function.
)
