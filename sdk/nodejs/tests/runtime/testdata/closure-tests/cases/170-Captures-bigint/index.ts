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

export const description = "Captures bigint";

// @ts-ignore
const zeroBigInt = 0n;
// @ts-ignore
const smallBigInt = 1n;
// @ts-ignore
const negativeBigInt = -1n;
// @ts-ignore
const largeBigInt = 11111111111111111111111111111111111111111n;
// @ts-ignore
const negativeLargeBigInt = -11111111111111111111111111111111111111111n;

export const func = function () { console.log(zeroBigInt + smallBigInt + negativeBigInt + largeBigInt + negativeBigInt + negativeLargeBigInt); };
