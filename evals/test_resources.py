import os
import subprocess
import unittest
from unittest import mock
from resources import Meter, profile, command_prefix


class ResourceTests(unittest.TestCase):
    def test_invalid_limits_fail_before_launch(self):
        for value in (0, -1, float("nan"), float("inf")):
            with self.assertRaises(ValueError):
                profile(memory_mib=value)

    def test_affinity_is_applied_to_child(self):
        limits = profile()
        result = subprocess.run(command_prefix(limits) + ["/usr/bin/python", "-c",
                     "import os; print(','.join(map(str, sorted(os.sched_getaffinity(0)))))"],
                     capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), ",".join(map(str, limits["cpus"])))

    def test_memory_overage_is_preserved_in_report(self):
        meter = Meter(profile(memory_mib=10))
        with mock.patch("resources.tree_sample", return_value={"rss_mib": 11, "live_cpu_seconds": 1, "processes": 2}):
            with self.assertRaisesRegex(RuntimeError, "memory"):
                meter.observe(os.getpid())
        self.assertEqual(meter.result()["peak_sampled_rss_mib"], 11)
        self.assertIsNotNone(meter.result()["limit_exceeded"])
