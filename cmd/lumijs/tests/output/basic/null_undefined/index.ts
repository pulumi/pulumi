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

// This test just ensures that undefined expands correctly.

let au = undefined;
let nu: number = undefined;
let su: string = undefined;

function f(x: string | undefined) {
    // Intentionally blank.
}

f(undefined);

let an = null;
let nn: number = null;
let sn: string = null;

function g(x: string | null) {
    // Intentionally blank.
}

g(null);

class C {
    pu: string | undefined;
    pn: number | null;
    constructor() {
        this.pu = undefined;
        this.pn = null;
    }
}

