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

import * as gstruct from "google-protobuf/google/protobuf/struct_pb";

// The magic prefix for encoded property values, matching the Go constant
// PropertyValueLogMagic = 0x7650696d756c7570 ("pulumiPv" little-endian).
const PROPERTY_VALUE_MAGIC = Buffer.from("pulumiPv", "ascii");

let logger: any = null;
let loggerProvider: any = null;

/**
 * Initialize the OTLP log exporter. Call this once during startup when
 * PULUMI_OTEL_EXPORTER_OTLP_ENDPOINT is set.
 */
export function initOtelLogging(endpoint: string): void {
    const { LoggerProvider, BatchLogRecordProcessor } = require("@opentelemetry/sdk-logs");
    const { OTLPLogExporter } = require("@opentelemetry/exporter-logs-otlp-grpc");
    const { Resource } = require("@opentelemetry/resources");
    const { ATTR_SERVICE_NAME } = require("@opentelemetry/semantic-conventions");
    const { credentials } = require("@grpc/grpc-js");

    const resource = Resource.default().merge(
        new Resource({
            [ATTR_SERVICE_NAME]: "pulumi-sdk-nodejs",
        }),
    );

    const exporter = new OTLPLogExporter({
        url: endpoint,
        credentials: credentials.createInsecure(),
    });
    const provider = new LoggerProvider({ resource });
    provider.addLogRecordProcessor(new BatchLogRecordProcessor(exporter));
    loggerProvider = provider;
    logger = provider.getLogger("pulumi-sdk-nodejs");
}

/**
 * Shut down the OTLP log provider, flushing any pending records.
 */
export async function shutdownOtelLogging(): Promise<void> {
    if (loggerProvider) {
        await loggerProvider.shutdown();
        loggerProvider = null;
        logger = null;
    }
}

/**
 * Wrapper that marks a value as a Pulumi property value. When passed
 * as an attribute to {@link emit}, it is automatically encoded using
 * the magic-prefix + protobuf format that the CLI's OTLP log receiver
 * understands.
 *
 * @example
 *   otelLogger.info("resource inputs", {
 *       urn: urn,
 *       inputs: new otelLogger.PropertyValue(serializedInputs),
 *   });
 */
export class PropertyValue {
    constructor(public readonly value: any) {}
}

/**
 * Encode a property value as bytes: [8-byte magic][protobuf Value].
 * Exported for testing; callers should normally use {@link PropertyValue}.
 */
export function encodePropertyValue(value: any): Uint8Array {
    const protoValue = gstruct.Value.fromJavaScript(value);
    const valueBytes = protoValue.serializeBinary();
    const buf = new Uint8Array(PROPERTY_VALUE_MAGIC.length + valueBytes.length);
    buf.set(PROPERTY_VALUE_MAGIC, 0);
    buf.set(valueBytes, PROPERTY_VALUE_MAGIC.length);
    return buf;
}

export enum Severity {
    TRACE = 1,
    DEBUG = 5,
    INFO = 9,
    WARN = 13,
    ERROR = 17,
}

/**
 * Emit a structured log record via OTLP. No-op when the logger hasn't
 * been initialized.
 *
 * Attribute values that are {@link PropertyValue} instances are
 * automatically encoded as bytes for the collector. All other values
 * pass through unchanged.
 */
export function emit(severity: Severity, message: string, attributes?: Record<string, unknown>): void {
    if (!logger) {
        return;
    }
    logger.emit({
        severityNumber: severity,
        body: message,
        attributes: processAttributes(attributes),
    });
}

function processAttributes(attrs?: Record<string, unknown>): Record<string, unknown> | undefined {
    if (!attrs) {
        return undefined;
    }
    const result: Record<string, unknown> = {};
    for (const [key, val] of Object.entries(attrs)) {
        if (val instanceof PropertyValue) {
            result[key] = encodePropertyValue(val.value);
        } else {
            result[key] = val;
        }
    }
    return result;
}

export function info(message: string, attributes?: Record<string, unknown>): void {
    emit(Severity.INFO, message, attributes);
}

export function debug(message: string, attributes?: Record<string, unknown>): void {
    emit(Severity.DEBUG, message, attributes);
}

export function warn(message: string, attributes?: Record<string, unknown>): void {
    emit(Severity.WARN, message, attributes);
}

export function error(message: string, attributes?: Record<string, unknown>): void {
    emit(Severity.ERROR, message, attributes);
}
