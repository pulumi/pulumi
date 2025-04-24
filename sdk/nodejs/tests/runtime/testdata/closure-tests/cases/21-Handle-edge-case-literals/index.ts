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

export const description = "Handle edge-case literals";

const a = -0;
const b = -0.0;
const c = Infinity;
const d = -Infinity;
const e = NaN;
const f = Number.MAX_SAFE_INTEGER;
const g = Number.MAX_VALUE;
const h = Number.MIN_SAFE_INTEGER;
const i = Number.MIN_VALUE;

export const func = () => { const x = [a, b, c, d, e, f, g, h, i]; };
