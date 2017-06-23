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

class C {}

// initialize empty maps of various kinds (implicit types).
let m1 = new Map<string, string>();
let m2 = new Map<string, number>();
let m3 = new Map<number, string>();
let m4 = new Map<string, C>();
let m5 = new Map<C, number>();
let m6 = new Map<C, string[]>();

// initialize empty maps of various kinds (explicit types).
let m11: Map<string, string> = new Map<string, string>();
let m12: Map<string, number> = new Map<string, number>();
let m13: Map<number, string> = new Map<number, string>();
let m14: Map<string, C> = new Map<string, C>();
let m15: Map<C, number> = new Map<C, number>();
let m16: Map<C, string[]> = new Map<C, string[]>();

// initialize maps using array constants.
let m7 = new Map<string, string>([
    [ "foo", "bar" ],
    [ "baz", "buz" ],
]);

