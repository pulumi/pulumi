# Copyright 2026, Pulumi Corporation.  All rights reserved.

import os
import signal
import time

# Ignore SIGINT so this process won't exit gracefully.
# This simulates a misbehaving program that doesn't handle cancellation.
signal.signal(signal.SIGINT, signal.SIG_IGN)

# Print our PID so the test can track us. The test waits for this line.
print(f"PID={os.getpid()}", flush=True)

# Sleep forever. The only way to stop us is SIGKILL.
while True:
    time.sleep(1)
