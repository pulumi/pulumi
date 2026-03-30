# Copyright 2026, Pulumi Corporation.

import os
import signal
import time

sentinel_dir = os.environ.get("SENTINEL_DIR", "")
if not sentinel_dir:
    raise Exception("SENTINEL_DIR not set")

with open(os.path.join(sentinel_dir, "started"), "w") as f:
    f.write("ok")

signal.signal(signal.SIGINT, signal.SIG_IGN)

time.sleep(3600)
