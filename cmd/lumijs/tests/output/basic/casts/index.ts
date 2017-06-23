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

// This tests out some good and bad cast cases.

let a: string = "x";
let b: any = <any>a; // ok.

// IDEA: a way to baseline expected failures.
// let c: number = <number>a; // statically rejected.

// IDEA: a way to baseline expected runtime failures.
// let d: number = <number>b; // dynamically rejected.

class C {}
let c: any = new C();
let isc: boolean = (c instanceof C);

