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

export const description = "Doesn't serialize non-free variables (but retains frees)";

let nocap1 = 1, nocap2 = 2, nocap3 = 3, nocap4 = 4, nocap5 = 5, nocap6 = 6, nocap7 = 7;
let nocap8 = 8, nocap9 = 9, nocap10 = 10;
let cap1 = 100, cap2 = 200, cap3 = 300, cap4 = 400, cap5 = 500, cap6 = 600, cap7 = 700;
let cap8 = 800;

const functext = `(nocap1, nocap2) => {
    let zz = nocap1 + nocap2; // not a capture: args
    let yy = nocap3; // not a capture: var later on
    if (zz) {
        zz += cap1; // true capture
        let cap1 = 9; // because let is properly scoped
        zz += nocap4; // not a capture
        var nocap4 = 7; // because var is function scoped
        zz += cap2; // true capture
        const cap2 = 33;
        var nocap3 = 8; // block the above capture
    }
    let f1 = (nocap5) => {
        yy += nocap5; // not a capture: args
        cap3++; // capture
    };
    let f2 = (function (nocap6) {
        zz += nocap6; // not a capture: args
        if (cap4) { // capture
            yy = 0;
        }
    });
    let www = nocap7(); // not a capture; it is defined below
    if (true) {
        function nocap7() {
        }
    }
    let [{t: [nocap8]},,nocap9 = "hello",...nocap10] = [{t: [true]},null,undefined,1,2];
    let vvv = [nocap8, nocap9, nocap10]; // not a capture; declarations from destructuring
    let aaa = { // captures in property and method declarations
        [cap5]: cap6,
        [cap7]() {
            cap8
        }
    }
}`;

export const func = eval(functext);