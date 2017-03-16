// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as webserver from "./webserver";

let wwwTierServer = new webserver.Micro("www");
let appTierServer = new webserver.Large("app");

