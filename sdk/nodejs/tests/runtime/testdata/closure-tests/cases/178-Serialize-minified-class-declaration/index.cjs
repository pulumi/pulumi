// Copyright 2025, Pulumi Corporation.
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

exports.description = "Serialize minified class declaration";

// important thing for this case is class declarations, and that spaces are not required, e.g after class

exports.func = () => {
    let y = 5;
    class foo{constructor(baz) {this.bar = baz;}get() {return this.bar}set(bing){this.bar = bing}}
    let x=new foo(1)
    x.set(y)
}
