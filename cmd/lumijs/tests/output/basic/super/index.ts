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

// This test ensures that references to `super` (base class) are resolved and emitted correctly.

class B {
    private x: number;

    constructor(x: number) {
        this.x = x;
    }

    public gx(): number {
        return this.x;
    }
}

class C extends B {
    constructor() {
        // This `super` should resolve to the constructor function:
        super(42);
    }

    public gy(): number {
        // This `super` should just be an object reference to the base object:
        let y = super.gx();
        return y;
    }
}

let b = new B(18);
let bgx = b.gx();
if (bgx !== 18) {
    throw new Error("Expected b.gx == 18; got " + bgx);
}

let c = new C();
let cgx = c.gx();
if (cgx !== 42) {
    throw new Error("Expected c.gx == 42; got " + cgx);
}
let cgy = c.gy();
if (cgy !== 42) {
    throw new Error("Expecred c.gy == 42; got " + cgy);
}

