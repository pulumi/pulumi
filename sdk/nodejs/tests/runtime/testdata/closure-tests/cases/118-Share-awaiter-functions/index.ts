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

export const description = "Share __awaiter functions";

const awaiter1 = function (thisArg: any, _arguments: any, P: any, generator: any) {
    return new (P || (P = Promise))(function (resolve: any, reject: any) {
        function fulfilled(value: any) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value: any) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result: any) { result.done ? resolve(result.value) : new P(function (resolve1: any) { resolve1(result.value); }).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
const awaiter2 = function (thisArg: any, _arguments: any, P: any, generator: any) {
    return new (P || (P = Promise))(function (resolve: any, reject: any) {
        function fulfilled(value: any) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value: any) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result: any) { result.done ? resolve(result.value) : new P(function (resolve1: any) { resolve1(result.value); }).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};

function f3() {
    const v1 = awaiter1, v2 = awaiter2;
}

export const func = f3;