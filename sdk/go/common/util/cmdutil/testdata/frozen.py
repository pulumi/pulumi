import signal
import sys
import time

def signal_handler(signal, frame):
    time.sleep(3)
    print("error: was not forced to exit", file=sys.stderr)
    sys.exit(2)

signal.signal(signal.SIGINT, signal_handler)
if hasattr(signal, "SIGBREAK"):
    # SIGBREAK is only available on Windows
    signal.signal(signal.SIGBREAK, signal_handler)
print("ready", flush=True)

time.sleep(3)
print("error: signal not received", file=sys.stderr)
sys.exit(1)
