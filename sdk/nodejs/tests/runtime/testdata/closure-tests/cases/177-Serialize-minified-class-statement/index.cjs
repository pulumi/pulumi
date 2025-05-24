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

exports.description = "Serialize minified class statement";

// important thing for this case is class expressions, and that spaces are not required, e.g after class

exports.func = () => {
    let z = [];
    let y=5,h=class{_fns;constructor(i=[]){this._fns=i;}clear(){this._fns=[];}exists(r){return this._fns.indexOf(r)!==-1}eject(r){let e=this._fns.indexOf(r);e!==-1&&(this._fns=[...this._fns.slice(0,e),...this._fns.slice(e+1)]);}use(r){this._fns=[...this._fns,r];}}
    new h(z).clear();
}
