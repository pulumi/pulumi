// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import { Config, runtime } from "../index";

describe("config", () => {
    it("works, basically", () => {
        // Set up some config and then read them back as strings.
        runtime.setConfig("pkg:config:a", "foo");
        runtime.setConfig("pkg:config:bar", "b");
        runtime.setConfig("pkg:config:baz", "baz");
        runtime.setConfig("otherpkg:config:a", "babble");
        runtime.setConfig("otherpkg:config:nothere", "bazzle");
        const config = new Config("pkg:config");
        assert.strictEqual("foo", config.get("a"));
        assert.strictEqual("foo", config.require("a"));
        assert.strictEqual("b", config.get("bar"));
        assert.strictEqual("b", config.require("bar"));
        assert.strictEqual("baz", config.get("baz"));
        assert.strictEqual("baz", config.require("baz"));
        assert.strictEqual(undefined, config.get("nothere"));
        assert.throws(() => { config.require("missing"); });
    });
    it("does strongly typed too!", () => {
        // Set up some config and then read them back as typed things.
        runtime.setConfig("pkg:config:boolf", "false");
        runtime.setConfig("pkg:config:boolt", "true");
        runtime.setConfig("pkg:config:num", "42.333");
        runtime.setConfig("pkg:config:array", "[ 0, false, 2, \"foo\" ]");
        runtime.setConfig("pkg:config:struct", "{ \"foo\": \"bar\", \"mim\": [] }");
        const config = new Config("pkg:config");
        assert.strictEqual(false, config.getBoolean("boolf"));
        assert.strictEqual(false, config.requireBoolean("boolf"));
        assert.strictEqual(true, config.getBoolean("boolt"));
        assert.strictEqual(true, config.requireBoolean("boolt"));
        assert.strictEqual(undefined, config.getBoolean("boolmissing"));
        assert.strictEqual(42.333, config.getNumber("num"));
        assert.strictEqual(42.333, config.requireNumber("num"));
        assert.strictEqual(undefined, config.getNumber("nummissing"));
        assert.deepEqual([ 0, false, 2, "foo" ], config.getObject<any>("array"));
        assert.deepEqual([ 0, false, 2, "foo" ], config.requireObject<any>("array"));
        assert.deepEqual({ "foo": "bar", "mim": [] }, config.getObject<any>("struct"));
        assert.deepEqual({ "foo": "bar", "mim": [] }, config.requireObject<any>("struct"));
        assert.strictEqual(undefined, config.getObject<any>("complexmissing"));
        // ensure requireX throws when missing:
        assert.throws(() => { config.requireBoolean("missing"); });
        assert.throws(() => { config.requireNumber("missing"); });
        assert.throws(() => { config.requireObject<any>("missing"); });
        // ensure getX throws when the value is of the wrong type:
        assert.throws(() => { config.getBoolean("num"); });
        assert.throws(() => { config.getNumber("boolf"); });
    });
});


