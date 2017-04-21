// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as coconut from "@coconut/coconut";
import * as mantle from "@coconut/mantle";

let func = new mantle.func.Function(
    "hellofunc",
    new coconut.asset.String(
        "exports.handler = function(e, ctx, cb) {\n" +
        "    cb(null, 'Hello, world!');\n" +
        "}\n",
    ),
    mantle.arch.runtimes.NodeJS,
);

let httpAPI = new mantle.http.API("/hello", "GET", func);

