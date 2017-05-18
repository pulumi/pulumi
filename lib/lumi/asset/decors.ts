// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// asset is a decorator function that can be used on a formal parameter to turn any expression into a code asset.  The
// presence of this decorator causes a Lumi compiler to skip all transformation of the target code.  Instead, the
// code will be emitted as a call to `new asset.Code(code)`, where `code` is the stringification of the expression text.
// TODO: this description is super naive, for instance, if we have captured variables.
// TODO: using a parameter decorator is controversial, since this is not formally part of ECMAScript TC39.
export function asset(target: Object, propertyKey: any, parameterIndex: number) {
}

