# Copyright 2026, Pulumi Corporation.

import json
import os
import signal
import sys
import time
from pathlib import Path

import pulumi.provider as provider

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


class Provider(provider.Provider):
    VERSION = "0.0.1"

    def __init__(self):
        super().__init__(Provider.VERSION, SCHEMA)

    def construct(self, name, resource_type, inputs, options=None):
        sentinel_dir = os.environ.get("SENTINEL_DIR", ".")

        # Write "started" sentinel to indicate construct has been entered.
        Path(os.path.join(sentinel_dir, "started")).write_text("started")

        # Ignore SIGINT
        signal.signal(signal.SIGINT, signal.SIG_IGN)
        # Block forever
        while True:
            time.sleep(1)


if __name__ == "__main__":
    provider.main(Provider(), sys.argv[1:])
