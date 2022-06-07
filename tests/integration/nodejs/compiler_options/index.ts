// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

// Confirm that Pulumi no longer forces the `target` `compilerOptions` in `tsconfig.json` to be `es6`.
//
// When `es6` is the target, if there isn't a version of the `@types/node` package available
// that declares `Array.prototype.includes`, using `includes` will result in a compiler error:
//
//     error TS2339: Property 'includes' does not exist on type 'number[]'.
//
// This test includes a resolution to force the use of an earlier version of `@types/node` that
// does not declare `Array.prototype.includes` (because it's a transitive dependency) and
// sets the `target` in `tsconfig.json` to `es2016`, which provides a declaration for `includes`.
// The program should run without any compiler errors.

if ([1, 2, 3].includes(1)) {
    console.log("hello world");
}
