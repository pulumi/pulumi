// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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

