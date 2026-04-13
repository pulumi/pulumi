# Copyright 2026, Pulumi Corporation.

import asyncio
import json
import os
import signal
import sys
from pathlib import Path

from pulumi.provider.experimental.provider import (
    ConstructRequest,
    ConstructResponse,
    GetSchemaRequest,
    GetSchemaResponse,
    Provider,
)
from pulumi.provider.experimental.server import main

SCHEMA = json.dumps(
    {
        "name": "test-provider",
        "version": "0.0.1",
        "resources": {
            "test-provider:index:Component": {
                "isComponent": True,
            },
        },
    }
)


class TestProvider(Provider):
    async def get_schema(self, request: GetSchemaRequest) -> GetSchemaResponse:
        return GetSchemaResponse(schema=SCHEMA)

    async def construct(self, request: ConstructRequest) -> ConstructResponse:
        sentinel_dir = os.environ.get("SENTINEL_DIR", ".")

        # Write "started" sentinel to indicate construct has been entered.
        Path(os.path.join(sentinel_dir, "started")).write_text("started")

        # Block forever, cooperatively.
        while True:
            await asyncio.sleep(1)

    # No cancel handler — this provider ignores cancellation.


if __name__ == "__main__":
    # Ignore SIGINT
    signal.signal(signal.SIGINT, signal.SIG_IGN)

    main(sys.argv[1:], "0.0.1", TestProvider())
