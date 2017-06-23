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

import * as lumi from "@lumi/lumi";
import * as mantle from "@lumi/mantle";

let func = new mantle.func.Function(
    "hellofunc",
    new lumi.asset.String(
        "exports.handler = function(e, ctx, cb) {\n" +
        "    cb(null, 'Hello, world!');\n" +
        "}\n",
    ),
    mantle.arch.runtimes.NodeJS,
);

let httpAPI = new mantle.http.API("/hello", "GET", func);

