import sys
import time

from nuxbt import Nuxbt, PRO_CONTROLLER


ADAPTER_PATH = sys.argv[1] if len(sys.argv) > 1 else "/org/bluez/hci1"
TIMEOUT_SECONDS = int(sys.argv[2]) if len(sys.argv) > 2 else 60

nx = Nuxbt(debug=True)
print("available", nx.get_available_adapters(), flush=True)

idx = nx.create_controller(PRO_CONTROLLER, ADAPTER_PATH)
print("created", idx, "on", ADAPTER_PATH, flush=True)

start = time.time()
last = None

while time.time() - start < TIMEOUT_SECONDS:
    state = nx.state[idx].copy()
    if state != last:
        print("state", state, flush=True)
        last = state

    if state.get("state") == "connected":
        print("CONNECTED", flush=True)
        sys.exit(0)

    if state.get("state") == "crashed":
        print("CRASHED", state.get("errors"), flush=True)
        sys.exit(2)

    time.sleep(1)

print("TIMEOUT", nx.state[idx], flush=True)
sys.exit(1)
