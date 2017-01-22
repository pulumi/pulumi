// Copyright 2016 Marapongo, Inc. All rights reserved.

package tokens

// Accessibility modifiers.
type Accessibility string // accessibility modifiers common to all.
const (
	PublicAccessibility  Accessibility = "public"
	PrivateAccessibility Accessibility = "private"
)

type ClassMemberAccessibility Accessibility // accessibility modifiers for class members.
const (
	PublicClassAccessibility    ClassMemberAccessibility = "public"
	PrivateClassAccessibility   ClassMemberAccessibility = "private"
	ProtectedClassAccessibility ClassMemberAccessibility = "protected"
)
