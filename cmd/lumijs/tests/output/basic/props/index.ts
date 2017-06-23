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

// This tests that class and module properties are emitted with correctly qualified token names.

// First, define some properties:

let modprop: string = "Foo";

class C {
    public static clastaprop: number = 42;
    public claprop: boolean = true;
    public get custprop(): string {
        return "getting a custom property";
    }
    public set custprop(v: string) {
    }
}

class D extends C {
    public cladprop: string = "yeah d!";
}

// Now create some references to those properties:

let a: string = modprop;
let b: number = C.clastaprop;
let c = new C();
if (c !== undefined) {
    let d: boolean = c.claprop;
    let e = {
        f: modprop,
        g: C.clastaprop,
        h: c.claprop,
        "i": "i",
        [j()]: "j",
    };
    let cust: string = c.custprop;
    c.custprop = "setting a custom property";
}

function j(): string { return "j"; }

// Define a local variable at the module's top-level within a block (should not be a module member).
{
    let notprop: string = "notprop";
    let notpropcop: string = notprop;
}

