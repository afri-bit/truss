import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import agent_gate as g  # noqa: E402

AGENTS = ["claude[bot]"]


def run(cwd, *a):
    subprocess.run(["git", *a], cwd=cwd, check=True, capture_output=True,
                   env={**os.environ, "GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.com",
                        "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.com"})


def write(root, rel, text=None):
    if text is None:
        text = "{}\n" if rel.endswith(".json") else "x\n"
    p = Path(root, rel)
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(text)


class GateTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = self.tmp.name
        run(self.root, "init", "-q", "-b", "develop")
        write(self.root, "README.md")
        run(self.root, "add", "-A")
        run(self.root, "commit", "-q", "-m", "init")

    def tearDown(self):
        self.tmp.cleanup()

    def branch(self, name):
        run(self.root, "switch", "-q", "-c", name)

    def commit(self, msg, *files):
        for f in files:
            write(self.root, f)
        run(self.root, "add", "-A")
        run(self.root, "commit", "-q", "-m", msg)

    def gate(self, branch, author="claude[bot]", chore=False, body=""):
        return g.evaluate(self.root, "develop", "HEAD", branch, author, AGENTS, chore, body)

    def test_bad_branch_name_fails(self):
        self.branch("chore/x")
        self.commit("c", "a.md")
        self.assertTrue(any(f.startswith("G1") for f in self.gate("chore/x", author="human")))

    def test_human_pr_skips_pipeline_checks(self):
        self.branch("feature/x")
        self.commit("c", "internal/a.go")
        self.assertEqual(self.gate("feature/x", author="human"), [])

    def test_agent_code_without_spec_fails(self):
        self.branch("feature/004-x")
        self.commit("c", "internal/a.go", ".agent/state/feature--004-x.json")
        self.assertTrue(any(f.startswith("G2") for f in self.gate("feature/004-x")))

    def test_agent_pipeline_passes(self):
        self.branch("feature/004-x")
        self.commit("spec", "specs/004-x/spec.md", "specs/004-x/plan.md", "specs/004-x/tasks.md",
                    ".agent/state/feature--004-x.json")
        self.commit("code", "internal/a.go")
        body = "Spec: specs/004-x/\nOpen tasks: none"
        self.assertEqual(self.gate("feature/004-x", body=body), [])

    def test_code_before_spec_fails(self):
        self.branch("feature/004-x")
        self.commit("code", "internal/a.go", ".agent/state/feature--004-x.json")
        self.commit("spec", "specs/004-x/spec.md", "specs/004-x/plan.md", "specs/004-x/tasks.md")
        fails = self.gate("feature/004-x", body="specs/004-x/\nOpen tasks: none")
        self.assertTrue(any(f.startswith("G3") for f in fails))

    def test_guardrail_path_fails(self):
        self.branch("feature/x")
        self.commit("c", ".github/workflows/ci.yml", ".agent/state/feature--x.json")
        self.assertTrue(any(f.startswith("G5") for f in self.gate("feature/x", chore=True)))

    def test_chore_label_allows_code_without_spec(self):
        self.branch("feature/rename")
        self.commit("c", "internal/a.go", ".agent/state/feature--rename.json")
        self.assertEqual(self.gate("feature/rename", chore=True), [])

    def test_missing_state_fails(self):
        self.branch("feature/rename")
        self.commit("c", "internal/a.go")
        self.assertTrue(any(f.startswith("G4") for f in self.gate("feature/rename", chore=True)))

    def test_missing_open_tasks_line_fails(self):
        self.branch("feature/004-x")
        self.commit("spec", "specs/004-x/spec.md", "specs/004-x/plan.md", "specs/004-x/tasks.md",
                    ".agent/state/feature--004-x.json")
        self.commit("code", "internal/a.go")
        fails = self.gate("feature/004-x", body="specs/004-x/")
        self.assertTrue(any("Open tasks" in f for f in fails))


if __name__ == "__main__":
    unittest.main()
