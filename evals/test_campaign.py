import copy
import unittest
from campaign import decide, objective


def report(passed=True):
    return {"status": "needs_review" if passed else "fail", "verification": {"checks": {
        "tests_unchanged_before_verification": True, "file_scope_respected": True,
        "independent_tests_pass": passed, "names_changed": True}}}


class CampaignTests(unittest.TestCase):
    def test_canary_requires_observed_improvement(self):
        self.assertEqual(decide([{"baseline": report(False), "candidate": report()}], 1)[0], "canary")
        self.assertEqual(decide([{"baseline": report(), "candidate": report()}], 1)[0], "retain")
        self.assertEqual(decide([], 1)[0], "retain")

    def test_regressions_and_integrity_failures_veto_wins(self):
        pairs = [{"baseline": report(False), "candidate": report()},
                 {"baseline": report(), "candidate": report(False)}]
        self.assertEqual(decide(pairs, 2)[0], "reject")
        bad = copy.deepcopy(report())
        bad["verification"]["checks"]["file_scope_respected"] = False
        self.assertEqual(decide([{"baseline": report(False), "candidate": bad}], 1)[0], "reject")
        self.assertFalse(objective({"status": "needs_review", "verification": {"checks": {}}}))

    def test_one_family_cannot_earn_broader_promotion(self):
        pair = {"family": "text", "baseline": report(False), "candidate": report()}
        self.assertEqual(decide([pair], 1, min_families=2)[0], "retain")

    def test_cost_regression_prevents_promotion(self):
        a, b = report(), report()
        a["journey_seconds"], b["journey_seconds"] = 10, 20
        pairs = [{"family": "text", "baseline": report(False), "candidate": report()},
                 {"family": "records", "baseline": a, "candidate": b}]
        self.assertEqual(decide(pairs, 2, min_families=2, resource_ratio=1.25)[0], "retain")

    def test_identical_binary_control_records_variation_without_claiming_code_regression(self):
        pair = {"baseline": report(), "candidate": report(False)}
        self.assertEqual(decide([pair], 1, same_binary=True)[0], "retain")
        pair["candidate"]["verification"]["checks"]["file_scope_respected"] = False
        self.assertEqual(decide([pair], 1, same_binary=True)[0], "reject")
