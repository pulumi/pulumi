// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as webserver from "./webserver";

let webServer = new webserver.Micro("www");
let appServer = new webserver.Nano("app");

