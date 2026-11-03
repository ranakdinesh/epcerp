"""Single-flight inference with a hard process deadline; never log request data."""
import os
import threading


class InferenceBusy(Exception):
    pass


class InferenceGuard:
    def __init__(self, seconds=10):
        self.seconds = seconds
        self.slot = threading.BoundedSemaphore(1)

    def run(self, work):
        if not self.slot.acquire(blocking=False):
            raise InferenceBusy()
        # ONNX is native code: a socket/client timeout does not stop computation.
        # Exit the isolated sidecar if it exceeds its budget. Docker restarts it
        # with the pinned model; no provider or database request is replayed here.
        watchdog = threading.Timer(self.seconds, lambda: os._exit(70))
        watchdog.daemon = True
        watchdog.start()
        try:
            return work()
        finally:
            watchdog.cancel()
            self.slot.release()
