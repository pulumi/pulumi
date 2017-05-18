// Copyright 2017 Pulumi, Inc. All rights reserved.

// asset is a decorator function that can be used on a formal parameter to turn any expression into a code asset.  The
// presence of this decorator causes a Lumi compiler to skip all transformation of the target code.  Instead, the
// code will be emitted as a call to `new asset.Code(code)`, where `code` is the stringification of the expression text.
// TODO: this description is super naive, for instance, if we have captured variables.
// TODO: using a parameter decorator is controversial, since this is not formally part of ECMAScript TC39.
export function asset(target: Object, propertyKey: any, parameterIndex: number) {
}

