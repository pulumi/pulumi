"use strict";
// Copyright 2016-2024, Pulumi Corporation.
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.componentProviderHost = componentProviderHost;
const fs_1 = require("fs");
const pulumi = __importStar(require("@pulumi/pulumi"));
const provider = __importStar(require("@pulumi/pulumi/provider"));
const schema_1 = require("./schema");
const instantiator_1 = require("./instantiator");
const path = __importStar(require("path"));
function getInputsFromOutputs(resource) {
    const result = {};
    for (const key of Object.keys(resource)) {
        const value = resource[key];
        if (pulumi.Output.isInstance(value)) {
            result[key] = value;
        }
    }
    return result;
}
class ComponentProvider {
    constructor(dir) {
        this.dir = dir;
        const absDir = path.resolve(dir);
        const packStr = (0, fs_1.readFileSync)(`${absDir}/package.json`, { encoding: "utf-8" });
        this.pack = JSON.parse(packStr);
        this.version = this.pack.version;
        this.path = absDir;
    }
    async getSchema() {
        const schema = (0, schema_1.generateSchema)(this.pack, this.path);
        return JSON.stringify(schema);
    }
    async construct(name, type, inputs, options) {
        const className = type.split(":")[2];
        const comp = await (0, instantiator_1.instantiateComponent)(this.path, className, name, inputs, options);
        return {
            urn: comp.urn,
            state: getInputsFromOutputs(comp),
        };
    }
}
function componentProviderHost(dirname) {
    const args = process.argv.slice(2);
    // If dirname is not provided, get it from the call stack
    if (!dirname) {
        // Get the stack trace
        const stack = new Error().stack;
        // Parse the stack to get the caller's file
        // Stack format is like:
        // Error
        //     at componentProviderHost (.../src/index.ts:3:16)
        //     at Object.<anonymous> (.../caller/index.ts:4:1)
        const callerLine = stack === null || stack === void 0 ? void 0 : stack.split('\n')[2];
        const match = callerLine === null || callerLine === void 0 ? void 0 : callerLine.match(/\((.+):[0-9]+:[0-9]+\)/);
        if (match && match[1]) {
            dirname = path.dirname(match[1]);
        }
        else {
            throw new Error('Could not determine caller directory');
        }
    }
    const prov = new ComponentProvider(dirname);
    return provider.main(prov, args);
}
//# sourceMappingURL=provider.js.map