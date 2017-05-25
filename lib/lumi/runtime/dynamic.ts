// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// isFunction checks whether the given object is a function (and hence invocable).
export function isFunction(obj: Object): boolean {
    return false; // functionality provided by the runtime.
}

// dynamicInvoke dynamically calls the target function.  If the target is not a function, an error is thrown.
export function dynamicInvoke(obj: Object, thisArg: Object, args: Object[]): Object {
    return <any>undefined; // functionality provided by the runtime.
}

// log prints the provided message to standard out.  
export function log(message: any): void {
    // functionality provided by the runtime.
}

