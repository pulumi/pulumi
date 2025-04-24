import sys
import time

print('ready', flush=True)

# HACK:
# time.sleep on Windows doesn't seem to be interruptible by signals
# so we'll sleep in small increments.
timeout = 3.0
while timeout > 0:
    time.sleep(0.05)
    timeout -= 0.05

print("error: was not terminated", file=sys.stderr)
sys.exit(1)
