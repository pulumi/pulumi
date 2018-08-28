// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as assert from "assert";
import { Config, runtime } from "../index";

describe("config", () => {
    it("works, basically", () => {
        // Set up some config and then read them back as strings.
        runtime.setConfig("pkg:a", "foo");
        runtime.setConfig("pkg:bar", "b");
        runtime.setConfig("pkg:baz", "baz");
        runtime.setConfig("otherpkg:a", "babble");
        runtime.setConfig("otherpkg:nothere", "bazzle");
        const config = new Config("pkg");
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
        runtime.setConfig("pkg:boolf", "false");
        runtime.setConfig("pkg:boolt", "true");
        runtime.setConfig("pkg:num", "42.333");
        runtime.setConfig("pkg:array", "[ 0, false, 2, \"foo\" ]");
        runtime.setConfig("pkg:struct", "{ \"foo\": \"bar\", \"mim\": [] }");
        const config = new Config("pkg");
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
    it("does enums", () => {
        runtime.setConfig("pkg:color", "orange");
        const config = new Config("pkg");
        assert.strictEqual("orange", config.getEnum("color", [ "purple", "orange", "blue" ]));
        assert.strictEqual(undefined, config.getEnum("missing", [ "purple", "orange", "blue" ]));
        assert.strictEqual("orange", config.requireEnum("color", [ "purple", "orange", "blue" ]));
        assert.throws(() => { config.getEnum("color", [ "purple", "black", "blue" ]); });
        assert.throws(() => { config.requireEnum("color", [ "purple", "black", "blue" ]); });
        assert.throws(() => { config.requireEnum("missing", [ "purple", "orange", "blue" ]); });
    });
    it("does min/max (strlen)", () => {
        runtime.setConfig("pkg:strlen", "abcdefgh");
        const config = new Config("pkg");
        assert.strictEqual("abcdefgh", config.getMinMax("strlen", 0, 8));
        assert.strictEqual("abcdefgh", config.getMinMax("strlen", 8, 8));
        assert.strictEqual("abcdefgh", config.getMinMax("strlen", 0, 16));
        assert.strictEqual(undefined, config.getMinMax("missing", 0, 8));
        assert.strictEqual("abcdefgh", config.requireMinMax("strlen", 0, 8));
        assert.strictEqual("abcdefgh", config.requireMinMax("strlen", 8, 8));
        assert.strictEqual("abcdefgh", config.requireMinMax("strlen", 0, 16));
        assert.throws(() => { config.getMinMax("strlen", 9, 16); });
        assert.throws(() => { config.getMinMax("strlen", 0, 7); });
        assert.throws(() => { config.requireMinMax("strlen", 9, 16); });
        assert.throws(() => { config.requireMinMax("strlen", 0, 7); });
        assert.throws(() => { config.requireMinMax("missing", 0, 8); });
    });
    it("does pattern matching", () => {
        runtime.setConfig("pkg:pattern", "aBcDeFgH");
        const config = new Config("pkg");
        assert.strictEqual("aBcDeFgH", config.getPattern("pattern", /^[a-zA-Z]*$/));
        assert.strictEqual("aBcDeFgH", config.getPattern("pattern", "^[a-zA-Z]*$"));
        assert.strictEqual(undefined, config.getPattern("missing", /^[a-zA-Z]*$/));
        assert.strictEqual("aBcDeFgH", config.requirePattern("pattern", /^[a-zA-Z]*$/));
        assert.strictEqual("aBcDeFgH", config.requirePattern("pattern", "^[a-zA-Z]*$"));
        assert.throws(() => { config.getPattern("pattern", /^[a-z]*$/); }, "bad pattern: get");
        assert.throws(() => { config.getPattern("pattern", "/^[a-z]*$/"); }, "bad pattern (string): get");
        assert.throws(() => { config.requirePattern("pattern", /^[a-z]*$/); }, "bad pattern: require");
        assert.throws(() => { config.requirePattern("pattern", "/^[a-z]*$/"); }, "bad pattern (string): require");
        assert.throws(() => { config.requirePattern("missing", /^[a-z]*$/); }, "missing");
    });
    it("does min/max (numbers)", () => {
        runtime.setConfig("pkg:quantity", "8");
        const config = new Config("pkg");
        assert.strictEqual(8, config.getNumberMinMax("quantity", 0, 8));
        assert.strictEqual(8, config.getNumberMinMax("quantity", 8, 8));
        assert.strictEqual(8, config.getNumberMinMax("quantity", 0, 16));
        assert.strictEqual(undefined, config.getNumberMinMax("missing", 0, 8));
        assert.strictEqual(8, config.requireNumberMinMax("quantity", 0, 8));
        assert.strictEqual(8, config.requireNumberMinMax("quantity", 8, 8));
        assert.strictEqual(8, config.requireNumberMinMax("quantity", 0, 16));
        assert.throws(() => { config.getNumberMinMax("quantity", 9, 16); });
        assert.throws(() => { config.getNumberMinMax("quantity", 0, 7); });
        assert.throws(() => { config.requireNumberMinMax("quantity", 9, 16); });
        assert.throws(() => { config.requireNumberMinMax("quantity", 0, 7); });
        assert.throws(() => { config.requireNumberMinMax("missing", 0, 8); });
    });
});
