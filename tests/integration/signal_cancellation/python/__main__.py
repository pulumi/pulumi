# Copyright 2026, Pulumi Corporation.

import os
import signal
import threading

sentinel_dir = os.environ.get("SENTINEL_DIR", "")
if not sentinel_dir:
    raise Exception("SENTINEL_DIR not set")

done = threading.Event()


def handle_sigint(signum, frame):
    with open(os.path.join(sentinel_dir, "graceful-shutdown"), "w") as f:
        f.write("ok")
    done.set()


signal.signal(signal.SIGINT, handle_sigint)

with open(os.path.join(sentinel_dir, "started"), "w") as f:
    f.write("ok")

done.wait()
