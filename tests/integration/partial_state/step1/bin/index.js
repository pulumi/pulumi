"use strict";
// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.
Object.defineProperty(exports, "__esModule", { value: true });
const resource_1 = require("./resource");
// resource "not-doomed" is updated, but the update partially fails.
const a = new resource_1.Resource("doomed", 4);
// "a" should still be in the checkpoint with its new value.
//# sourceMappingURL=index.js.map