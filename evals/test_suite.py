from pathlib import Path
import tempfile
import unittest
import run as base
from suite import scenario, public_scenario, FAMILIES
from grading import isolated_verify
from terminal import allowed, available

SOLUTIONS = {
    "text": 'def clean_text(value): return " ".join(value.split()).lower()\n',
    "records": 'def count_items(value): return {x:value.count(x) for x in value}\n',
    "paths": 'def file_suffix(value):\n n=value.rsplit("/",1)[-1]\n return "" if n.startswith(".") or "." not in n else n.rsplit(".",1)[-1].lower()\n',
}


class SuiteTests(unittest.TestCase):
    def test_private_values_are_not_in_worker_scenario(self):
        for family in FAMILIES:
            case = scenario(family, 42)
            self.assertNotIn("private_inputs", public_scenario(case))
            self.assertNotIn("private_expected", public_scenario(case))
            self.assertEqual(case, scenario(family, 42))
            self.assertNotEqual(case["variant_id"], scenario(family, 43)["variant_id"])

    def test_family_permissions_follow_generated_files(self):
        case = scenario("records", 42)
        workspace = Path("/tmp/fixture")
        self.assertTrue(allowed("write", {"file_path": case["allowed_files"][0]}, workspace, True,
                                implementation=case["allowed_files"][0]))
        self.assertFalse(allowed("write", {"file_path": "names.py"}, workspace, True,
                                implementation=case["allowed_files"][0]))

    def test_correct_implementations_pass_and_public_example_overfit_fails(self):
        try:
            available()
        except RuntimeError as error:
            self.skipTest(str(error))
        for family in FAMILIES:
            with self.subTest(family=family), tempfile.TemporaryDirectory() as directory:
                case = scenario(family, 42)
                work = Path(directory) / "workspace"
                work.mkdir()
                for name, content in case["files"].items():
                    (work / name).write_text(content)
                before = base.snapshot(work)
                impl = work / case["allowed_files"][0]
                impl.write_text(SOLUTIONS[family])
                self.assertTrue(all(isolated_verify(base, work, case, before)["checks"].values()))
                # Memorizing every public example still fails fresh values.
                public = {"text": ["  Hello   WORLD  ", "", "A\tB\nC"],
                          "records": [["a", "b", "a"], [], ["A", "a"]],
                          "paths": ["docs/Report.PDF", "folder.dot/readme", ".config"]}[family]
                oracle = {}
                namespace = {}
                exec(SOLUTIONS[family], namespace)
                for value in public:
                    oracle[repr(value)] = namespace[case["function"]](value)
                impl.write_text(f"def {case['function']}(value): return {oracle!r}.get(repr(value))\n")
                self.assertFalse(isolated_verify(base, work, case, before)["checks"]["hidden_checks_pass"])
