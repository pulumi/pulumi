// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Define a bunch of no-op decorators.
export function classDecorate(target: Object) {}
export function propertyDecorate(target: Object, propertyKey: string) {}
export function methodDecorate(target: Object, propertyKey: any, descriptor: any) {}
export function parameterDecorate(target: Object, propertyKey: string, parameterIndex: number) {}

