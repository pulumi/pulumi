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

export const description = "Capture factory func #2";

const outerVal = [{}];
(<any>outerVal[0]).inner = outerVal;

function foo() {
    outerVal.pop();
}

function bar() {
    outerVal.join();
}

export const func = () => {
    outerVal.push({});
    foo();

    return (event: any, context: any) => {
        bar();
    };
};

export const isFactoryFunction = true;