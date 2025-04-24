// Copyright 2024-2024, Pulumi Corporation.
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

export const description = "Capture all props if prototype is accessed #2.1";

class C {
    a: number;

    constructor() {
        this.a = 1;
    }

    m() { (<any>this).n(); }
}

class D extends C {
    b: number;
    constructor() {
        super();
        this.b = 2;
    }
    n() { }
}
const o = new D();

export const func = function () { o["m"](); };
