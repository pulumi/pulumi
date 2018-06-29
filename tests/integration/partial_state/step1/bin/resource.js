"use strict";
// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
const dynamic = require("@pulumi/pulumi/dynamic");
class Provider {
    constructor() {
        this.id = 0;
    }
    check(olds, news) {
        return __awaiter(this, void 0, void 0, function* () {
            return {
                inputs: news,
            };
        });
    }
    create(inputs) {
        return __awaiter(this, void 0, void 0, function* () {
            if (inputs.state === 4) {
                return Promise.reject("state can't be 4");
            }
            return {
                id: (this.id++).toString(),
                outs: inputs,
            };
        });
    }
    update(id, olds, news) {
        return __awaiter(this, void 0, void 0, function* () {
            if (news.state === 4) {
                return Promise.reject("state can't be 4");
            }
            return {
                outs: news,
            };
        });
    }
}
Provider.instance = new Provider();
exports.Provider = Provider;
class Resource extends dynamic.Resource {
    constructor(name, num, opts) {
        super(Provider.instance, name, { state: num }, opts);
    }
}
exports.Resource = Resource;
//# sourceMappingURL=resource.js.map