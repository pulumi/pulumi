import signal
import sys
import time

def signal_handler(signal, frame):
    print("Got SIGINT, cleaning up...")
    time.sleep(1)
    sys.exit(0)

signal.signal(signal.SIGINT, signal_handler)
signal.signal(signal.SIGTERM, signal_handler)
print("Waiting for SIGINT...")

time.sleep(3)
print("No SIGINT received.")
sys.exit(1)
