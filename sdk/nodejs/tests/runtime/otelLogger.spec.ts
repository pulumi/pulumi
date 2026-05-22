// Copyright 2026, Pulumi Corporation.
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
import * as gstruct from "google-protobuf/google/protobuf/struct_pb";
import { encodePropertyValue, PropertyValue, emit, Severity } from "../../runtime/otelLogger";
import { specialSigKey, specialSecretSig } from "../../runtime/rpc";

describe("otelLogger", () => {
    describe("encodePropertyValue", () => {
        it("encodes with the correct magic prefix", () => {
            const encoded = encodePropertyValue("hello");
            const magic = Buffer.from(encoded.slice(0, 8)).toString("ascii");
            assert.strictEqual(magic, "pulumiPv");
        });

        it("round-trips a simple value through protobuf", () => {
            const input = { name: "my-bucket", count: 42 };
            const encoded = encodePropertyValue(input);

            // Skip the 8-byte magic and deserialize the rest as a protobuf Value.
            const valueBytes = encoded.slice(8);
            const protoValue = gstruct.Value.deserializeBinary(valueBytes);
            const decoded = protoValue.toJavaScript();

            assert.deepStrictEqual(decoded, input);
        });

        it("preserves secret signatures", () => {
            const input = {
                name: "my-bucket",
                password: {
                    [specialSigKey]: specialSecretSig,
                    value: "hunter2",
                },
            };
            const encoded = encodePropertyValue(input);

            const valueBytes = encoded.slice(8);
            const protoValue = gstruct.Value.deserializeBinary(valueBytes);
            const decoded = protoValue.toJavaScript() as Record<string, any>;

            assert.strictEqual(decoded.name, "my-bucket");
            assert.strictEqual(decoded.password[specialSigKey], specialSecretSig);
            assert.strictEqual(decoded.password.value, "hunter2");
        });

        it("handles arrays", () => {
            const input = [1, "two", true];
            const encoded = encodePropertyValue(input);

            const valueBytes = encoded.slice(8);
            const protoValue = gstruct.Value.deserializeBinary(valueBytes);
            const decoded = protoValue.toJavaScript();

            assert.deepStrictEqual(decoded, input);
        });

        it("handles primitives", () => {
            for (const input of ["hello", 42, true, null]) {
                const encoded = encodePropertyValue(input);
                const valueBytes = encoded.slice(8);
                const protoValue = gstruct.Value.deserializeBinary(valueBytes);
                assert.deepStrictEqual(protoValue.toJavaScript(), input);
            }
        });
    });

    describe("PropertyValue wrapper", () => {
        it("wraps a value for auto-encoding", () => {
            const pv = new PropertyValue({ key: "val" });
            assert.deepStrictEqual(pv.value, { key: "val" });
        });
    });

    describe("wire format compatibility with Go receiver", () => {
        it("magic prefix is exactly 8 bytes matching the Go constant", () => {
            const encoded = encodePropertyValue("test");
            // Go: PropertyValueLogMagic = 0x7650696d756c7570
            // That's "pulumiPv" in ASCII, stored as LE uint64.
            const magicBytes = encoded.slice(0, 8);
            assert.strictEqual(Buffer.from(magicBytes).toString("ascii"), "pulumiPv");

            // Verify the LE uint64 matches: read as LE and compare.
            const view = new DataView(magicBytes.buffer, magicBytes.byteOffset, 8);
            const lo = view.getUint32(0, true);
            const hi = view.getUint32(4, true);
            // 0x7650696d756c7570 → lo=0x756c7570, hi=0x7650696d
            assert.strictEqual(lo, 0x756c7570);
            assert.strictEqual(hi, 0x7650696d);
        });

        it("bytes after magic are a valid google.protobuf.Value", () => {
            const input = {
                name: "test",
                nested: { [specialSigKey]: specialSecretSig, value: "secret" },
            };
            const encoded = encodePropertyValue(input);
            const valueBytes = encoded.slice(8);

            // Must not throw — the Go side does proto.Unmarshal(data[8:], &structpb.Value{})
            const decoded = gstruct.Value.deserializeBinary(valueBytes);
            const js = decoded.toJavaScript() as Record<string, any>;
            assert.strictEqual(js.name, "test");
            assert.strictEqual(js.nested[specialSigKey], specialSecretSig);
        });
    });

    describe("emit", () => {
        it("is a no-op when logger is not initialized", () => {
            // Should not throw even without initOtelLogging.
            emit(Severity.INFO, "test message", {
                key: "value",
                pv: new PropertyValue({ secret: "data" }),
            });
        });
    });
});
