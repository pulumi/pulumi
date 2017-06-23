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

// initialize an empty array.
let a1 = [];

// initialize an array of three strings (inferred).
let a2 = [ "a", "b", "c" ];
let a2a = a2[0];
a2[0] = "x";
let a2b = a2[1];
a2[1] = "x";
let a2c = a2[2];
a2[2] = "x";

// initialize an array of three strings (explicitly typed).
let a3: string[] = [ "a", "b", "c" ];
let a3a: string = a3[0];
let a3b: string = a3[0];
let a3c: string = a3[0];

// initialize a heterogeneously typed array (inferred).
let a4 = [ 0, true, "a", {} ];

// initialize a heterogeneously typed array (explicitly typed).
let a5: any[] = [ 0, true, "a", {} ];

// initialize an array of arrays (inferred).
let a6 = [
    [ "a", "b", "c" ],
    [ "x", "y", "z" ],
];
let a60 = a6[0];
let a60a = a6[0][0];
let a60b = a6[0][1];
let a60c = a6[0][2];
let a61 = a6[1];
let a61x = a6[1][0];
let a61y = a6[1][1];
let a61z = a6[1][2];

// initialize an array of arrays (explicitly typed).
let a7: string[][] = [
    [ "a", "b", "c" ],
    [ "x", "y", "z" ],
];
let a70: string[] = a6[0];
let a70a: string = a6[0][0];
let a70b: string = a6[0][1];
let a70c: string = a6[0][2];
let a71: string[] = a6[1];
let a71x: string = a6[1][0];
let a71y: string = a6[1][1];
let a71z: string = a6[1][2];

