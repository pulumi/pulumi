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

export const description = "Serializes basic captures";

const wcap = "foo";
const xcap = 97;
const ycap = [true, -1, "yup"];
const zcap = {
    a: "a",
    b: false,
    c: [0],
};

export const func = () => { console.log(wcap + `${xcap}` + ycap.length + eval(zcap.a + zcap.b + zcap.c)); };
