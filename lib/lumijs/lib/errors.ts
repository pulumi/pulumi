// Copyright 2016-2017, Pulumi Corporation
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

// Error objects are thrown when ECMAScript runtime errors occur. The Error class can also be used as a base for user-
// defined exceptions. See https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Error/.
export class Error {
    public name: string;
    public message: string | undefined;
    constructor(message?: string) {
        this.name = "Error";
        this.message = message;
    }
}

// TODO[pulumi/mu#70]: consider projecting all of the various subclasses (EvalError, RangeError, ReferenceError,
//     SyntaxError, TypeError, etc.)  Unfortunately, unless we come up with some clever way of mapping Lumi runtime
//     errors into their ECMAScript equivalents, we aren't going to have perfect compatibility with error path logic.

export class TypeError extends Error {
    constructor(message?: string) {
        super(message);
        this.name = "TypeError";
    }
}

