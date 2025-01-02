"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.willThrow = willThrow;
var pulumi = require("@pulumi/pulumi");
function willThrow() {
    if (true) {
        pulumi.log.error("Oh no!");
        throw new Error("this is a test error");
    }
}
willThrow();
//# sourceMappingURL=index.js.map