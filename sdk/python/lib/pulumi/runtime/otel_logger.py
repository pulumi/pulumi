# Copyright 2026, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
OTLP structured logging for Pulumi Python SDK.

Provides functions to emit log records via OTLP to the CLI's log
receiver. Property values can be sent as encoded attributes that the
receiver decodes for redaction and display.

Usage::

    from pulumi.runtime import otel_logger

    otel_logger.info("resource inputs", {
        "urn": urn,
        "inputs": otel_logger.PropertyValue(serialized_inputs),
    })
"""

from __future__ import annotations

import os
from typing import Any, Optional

from google.protobuf import json_format, struct_pb2

# The magic prefix for encoded property values, matching the Go constant
# PropertyValueLogMagic = 0x7650696d756c7570 ("pulumiPv" little-endian).
_PROPERTY_VALUE_MAGIC = b"pulumiPv"

_logger: Any = None
_logger_provider: Any = None


class PropertyValue:
    """Wrapper that marks a value as a Pulumi property value.

    When passed as an attribute value to :func:`emit`, the value is
    automatically encoded using the magic-prefix + protobuf format
    that the CLI's OTLP log receiver understands.
    """

    __slots__ = ("value",)

    def __init__(self, value: Any) -> None:
        self.value = value


def encode_property_value(value: Any) -> bytes:
    """Encode a property value as bytes: [8-byte magic][protobuf Value].

    The *value* should be a Python object in the Pulumi wire format
    (dicts with secret signatures, lists, primitives).  It is converted
    to a ``google.protobuf.Value``, serialized, and prefixed with the
    magic bytes.
    """
    proto_value = struct_pb2.Value()
    json_format.ParseDict(value, proto_value)
    return _PROPERTY_VALUE_MAGIC + proto_value.SerializeToString()


def init_otel_logging(endpoint: str) -> None:
    """Initialize the OTLP log exporter.

    Called once during startup when ``PULUMI_OTEL_EXPORTER_OTLP_ENDPOINT``
    is set.  The exporter sends log records to the CLI's OTLP receiver
    over insecure gRPC.
    """
    global _logger, _logger_provider  # noqa: PLW0603

    from opentelemetry.exporter.otlp.proto.grpc._log_exporter import (
        OTLPLogExporter,
    )
    from opentelemetry.sdk._logs import LoggerProvider
    from opentelemetry.sdk._logs.export import BatchLogRecordProcessor
    from opentelemetry.sdk.resources import Resource, SERVICE_NAME

    resource = Resource.create({SERVICE_NAME: "pulumi-sdk-python"})
    exporter = OTLPLogExporter(endpoint=endpoint, insecure=True)
    provider = LoggerProvider(resource=resource)
    provider.add_log_record_processor(BatchLogRecordProcessor(exporter))
    _logger_provider = provider
    _logger = provider.get_logger("pulumi-sdk-python")


def shutdown_otel_logging() -> None:
    """Shut down the OTLP log provider, flushing pending records."""
    global _logger, _logger_provider  # noqa: PLW0603
    if _logger_provider is not None:
        _logger_provider.shutdown()
        _logger_provider = None
        _logger = None


def _process_attributes(
    attrs: Optional[dict[str, Any]],
) -> Optional[dict[str, Any]]:
    if attrs is None:
        return None
    result: dict[str, Any] = {}
    for key, val in attrs.items():
        if isinstance(val, PropertyValue):
            result[key] = encode_property_value(val.value)
        else:
            result[key] = val
    return result


# Severity numbers matching the OTLP spec.
SEVERITY_TRACE = 1
SEVERITY_DEBUG = 5
SEVERITY_INFO = 9
SEVERITY_WARN = 13
SEVERITY_ERROR = 17


def emit(
    severity: int, message: str, attributes: Optional[dict[str, Any]] = None
) -> None:
    """Emit a structured log record via OTLP.

    No-op when the logger hasn't been initialized.

    :param severity: OTLP severity number (use the ``SEVERITY_*`` constants).
    :param message: The log message body.
    :param attributes: Key-value attributes.  Values that are
        :class:`PropertyValue` instances are automatically encoded as
        bytes for the collector.
    """
    if _logger is None:
        return
    from opentelemetry._logs import SeverityNumber
    from opentelemetry.sdk._logs import LogRecord
    from opentelemetry.trace import TraceFlags

    _logger.emit(
        LogRecord(
            severity_number=SeverityNumber(severity),
            body=message,
            attributes=_process_attributes(attributes),
            trace_id=0,
            span_id=0,
            trace_flags=TraceFlags(0),
        )
    )


def info(message: str, attributes: Optional[dict[str, Any]] = None) -> None:
    emit(SEVERITY_INFO, message, attributes)


def debug(message: str, attributes: Optional[dict[str, Any]] = None) -> None:
    emit(SEVERITY_DEBUG, message, attributes)


def warn(message: str, attributes: Optional[dict[str, Any]] = None) -> None:
    emit(SEVERITY_WARN, message, attributes)


def error(message: str, attributes: Optional[dict[str, Any]] = None) -> None:
    emit(SEVERITY_ERROR, message, attributes)
