// Copyright 2016 Marapongo, Inc. All rights reserved.

// isFunction checks whether the given object is a function (and hence invocable).
export function isFunction(obj: Object): boolean {
    return false; // functionality provided by the runtime.
}

// dynamicInvoke dynamically calls the target function.  If the target is not a function, an error is thrown.
export function dynamicInvoke(obj: Object, thisArg: Object, args: Object[]): Object {
    return <any>undefined; // functionality provided by the runtime.
}

