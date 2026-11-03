import subprocess
import sys
import threading
import time
import unittest
from pathlib import Path
from guardrails import InferenceBusy, InferenceGuard


class GuardTests(unittest.TestCase):
    def test_busy_request_rejected_without_queueing(self):
        entered, release = threading.Event(), threading.Event()
        guard = InferenceGuard(3)
        thread = threading.Thread(target=lambda: guard.run(lambda: (entered.set(), release.wait())))
        thread.start()
        try:
            self.assertTrue(entered.wait(1))
            with self.assertRaises(InferenceBusy):
                guard.run(lambda: self.fail('busy inference was executed'))
        finally:
            release.set()
            thread.join(1)
        self.assertEqual(guard.run(lambda: 42), 42)

    def test_wedged_inference_terminates_process(self):
        start = time.monotonic()
        p = subprocess.run([sys.executable, '-c',
            'from guardrails import InferenceGuard; import time; '
            'InferenceGuard(.05).run(lambda: time.sleep(30))'],
            cwd=Path(__file__).parent, timeout=3)
        self.assertEqual(p.returncode, 70)
        self.assertLess(time.monotonic() - start, 2)

    def test_completed_work_does_not_trigger_watchdog(self):
        guard = InferenceGuard(.05)
        self.assertEqual(guard.run(lambda: 7), 7)
        time.sleep(.1)


if __name__ == '__main__':
    unittest.main()
