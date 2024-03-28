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

// @ts-ignore
import { output } from "@pulumi/pulumi";

export const description = "Capturing same value through outputs multiple times";

const x = { a: 1, b: true };

const o1 = output(x);
const o2 = output(x);

const y = { o1, o2 };
const o3 = output(y);
const o4 = output(y);

const o5: any = { o3, o4 };

o5.a = output(y);
o5.b = y;
o5.c = [output(y)];
o5.d = [y];

o5.a_1 = o5.a;
o5.b_1 = o5.b;
o5.c_1 = o5.c;
o5.d_1 = o5.d;

const o6 = output(o5);

const v = { x, o1, o2, y, o3, o4, o5, o6 };

export const func = function () { console.log(v); };
