// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as webserver from "./webserver";

let webServer = new webserver.Micro("www");
let appServer = new webserver.Large("app");

