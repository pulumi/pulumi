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

export const description = "Complex capturing cases #1";

let nocap1 = 1;
let cap1 = 100;

export const func = () => {
    // cap1 is captured here.
    // nocap1 introduces a new variable that shadows the outer one.
    let [nocap1 = cap1] = [];
    console.log(nocap1);
};
