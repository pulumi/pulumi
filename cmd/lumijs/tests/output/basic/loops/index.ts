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

// Test out the various kinds of loops.

// test some loops within a function.
function loops() {
    let x = 0;
    while (x < 10) {
        x++;
    }
    if (x !== 10) {
        throw "Expected x == 10";
    }

    let last = false;
    for (let i = 0; i < 10; i++) {
        if (i === 9) {
            last = true;
        }
    }
    if (!last) {
        throw "Expected last == true";
    }
}

// now test those same loops at the module's top-level.
let x = 0;
while (x < 10) {
    x++;
}
if (x !== 10) {
    throw "Expected x == 10";
}

let last = false;
for (let i = 0; i < 10; i++) {
    if (i === 9) {
        last = true;
    }
}
if (!last) {
    throw "Expected last == true";
}

