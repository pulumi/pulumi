import signal
import sys
import time

def signal_handler(signal, frame):
    print("exiting cleanly", flush=True)
    sys.exit(0)

signal.signal(signal.SIGINT, signal_handler)
if hasattr(signal, "SIGBREAK"):
    # SIGBREAK is only available on Windows
    signal.signal(signal.SIGBREAK, signal_handler)
print("ready", flush=True)

# HACK:
# time.sleep on Windows doesn't seem to be interruptible by signals
# so we'll sleep in small increments.
timeout = 3.0
while timeout > 0:
    time.sleep(0.05)
    timeout -= 0.05

print("error: signal not received", file=sys.stderr)
sys.exit(1)
