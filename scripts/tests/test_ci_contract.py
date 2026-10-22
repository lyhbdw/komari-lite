#!/usr/bin/env python3
"""Local contract tests for the reusable GitHub Actions CI gate.

PyYAML is used when available. A small standard-library parser covers the
workflow keys asserted here so a missing optional dependency cannot turn into
a false contract failure. The tests never contact GitHub or execute Actions.
"""
from pathlib import Path
import re
import unittest

try:
    import yaml
except ImportError:  # pragma: no cover - exercised on minimal hosts
    yaml = None

REPO = Path(__file__).resolve().parents[2]
WORKFLOWS = REPO / ".github" / "workflows"


def _scalar_list(value):
    return re.findall(r"[A-Za-z0-9_-]+", value)


def _fallback_parse(text):
    """Parse the small YAML subset used by these workflow contracts."""
    lines = text.splitlines()
    jobs = {}
    in_jobs = False
    current = None
    blocks = {}
    for line in lines:
        if line == "jobs:":
            in_jobs = True
            continue
        if not in_jobs:
            continue
        match = re.match(r"^  ([A-Za-z0-9_-]+):\s*$", line)
        if match:
            current = match.group(1)
            blocks[current] = []
            continue
        if current is not None:
            blocks[current].append(line)
    for name, block in blocks.items():
        value = {"needs": set(), "steps": []}
        block_text = "\n".join(block)
        needs = re.search(r"^    needs:\s*(.+)$", block_text, re.MULTILINE)
        if needs:
            raw = needs.group(1).strip()
            value["needs"] = set(_scalar_list(raw)) if raw.startswith("[") else {raw}
        step = None
        for line in block:
            if re.match(r"^      - name:", line):
                step = {}
                value["steps"].append(step)
            elif step is not None:
                uses = re.match(r"^        uses:\s*(.+)$", line)
                run = re.match(r"^        run:\s*(.+)$", line)
                artifact = re.match(r"^          artifact-name:\s*(.+)$", line)
                if uses:
                    step["uses"] = uses.group(1).strip()
                if run:
                    step["run"] = run.group(1).strip()
                if artifact:
                    step.setdefault("with", {})["artifact-name"] = artifact.group(1).strip().strip('"')
        jobs[name] = value
    trigger = {}
    for event in ("pull_request", "push"):
        match = re.search(rf"(?ms)^  {event}:\n(.*?)(?=^  [A-Za-z_]|^\npermissions:)", text)
        if match:
            trigger[event] = {"branches": re.findall(r"^      - (.+)$", match.group(1), re.MULTILINE)}
    if re.search(r"(?m)^  workflow_call:\s*$", text):
        trigger["workflow_call"] = None
    return {"on": trigger, "jobs": jobs}


class WorkflowContractTests(unittest.TestCase):
    def read(self, name):
        return (WORKFLOWS / name).read_text()

    def parsed(self, name):
        text = self.read(name)
        if yaml is not None:
            data = yaml.safe_load(text)
        else:
            data = _fallback_parse(text)
        self.assertIsInstance(data, dict)
        self.assertIsInstance(data.get("jobs"), dict)
        self.assertTrue(data["jobs"], "workflow must define runnable jobs")
        return data

    def job_needs(self, data, job):
        value = data["jobs"][job].get("needs", [])
        if isinstance(value, str):
            return {value}
        return set(value)

    @unittest.skipUnless(yaml is not None, "Full YAML syntax validation needs optional PyYAML; standard-library contract checks still run")
    def test_workflows_parse_as_yaml(self):
        for path in WORKFLOWS.glob("*.yml"):
            with self.subTest(workflow=path.name):
                self.parsed(path.name)

    def test_gate_has_required_triggers(self):
        data = self.parsed("ci-gate.yml")
        triggers = data.get(True, data.get("on", {}))
        self.assertIn("pull_request", triggers)
        self.assertIn("push", triggers)
        self.assertIn("workflow_call", triggers)
        self.assertEqual(triggers["pull_request"]["branches"], ["main", "dev"])
        self.assertEqual(triggers["push"]["branches"], ["main", "dev"])

    def test_gate_tests_root_and_agent_modules(self):
        text = self.read("ci-gate.yml")
        data = self.parsed("ci-gate.yml")
        self.assertIn("root-go", data["jobs"])
        self.assertIn('CGO_ENABLED: "1"', text)
        self.assertIn("-tags nomsgpack,sqlite_omit_load_extension", text)
        self.assertIn("go test -tags nomsgpack,sqlite_omit_load_extension ./ ./cmd/... ./database/... ./internal/... ./pkg/... ./utils/... ./web/... ./protocol/...", text)
        self.assertIn("working-directory: agent", text)
        self.assertIn('CGO_ENABLED: "0"', text)
        self.assertRegex(text, r"(?m)^\s+run: go test ./\.\.\.\s*$")
        self.assertIn("go build -trimpath -ldflags=\"-s -w\"", text)
        self.assertIn("go test -race ./...", text)

    def test_gate_runs_its_own_contracts(self):
        text = self.read("ci-gate.yml")
        self.assertIn("run: bash scripts/check-ci-contract.sh", text)

    def test_every_release_build_uses_production_sqlite_tags(self):
        for filename in ("build.yml", "development.yml", "release.yml", "snapshot.yml", "release-docker.yml", "docker-publish.yml"):
            for line in self.read(filename).splitlines():
                if "go build " in line:
                    with self.subTest(workflow=filename, command=line.strip()):
                        self.assertIn("-tags nomsgpack,sqlite_omit_load_extension", line)

    def test_gate_tests_and_typebuilds_both_frontends(self):
        text = self.read("ci-gate.yml")
        self.assertIn("working-directory: frontend", text)
        self.assertIn("working-directory: theme", text)
        self.assertEqual(text.count("run: npm test"), 2)
        self.assertIn("uses: ./.github/actions/build-frontend", text)
        action = (REPO / ".github/actions/build-frontend/action.yml").read_text()
        self.assertEqual(action.count("npm run build"), 2)

    def test_root_tests_restore_real_embedded_frontends(self):
        data = self.parsed("ci-gate.yml")
        self.assertIn("frontend", self.job_needs(data, "root-go"))
        steps = data["jobs"]["root-go"]["steps"]
        restore = next(i for i, s in enumerate(steps) if s.get("uses") == "./.github/actions/restore-frontend")
        test = next(i for i, s in enumerate(steps) if s.get("run", "").startswith("go test"))
        self.assertLess(restore, test)
        self.assertEqual(steps[restore]["with"]["artifact-name"], "ci-frontend")

    def test_release_and_snapshot_publish_paths_need_gate(self):
        expectations = {
            "release.yml": {"build-and-attach-binaries", "build-and-push-docker"},
            "snapshot.yml": {"build-and-attach-binaries", "publish-snapshot-release", "build-and-push-docker"},
            "release-docker.yml": {"build-docker-binaries", "build-and-push-multi-arch"},
            "docker-publish.yml": {"build-docker-binaries", "build-and-push-multi-arch"},
        }
        for filename, jobs in expectations.items():
            data = self.parsed(filename)
            for job in jobs:
                with self.subTest(workflow=filename, job=job):
                    self.assertIn(job, data["jobs"])
                    self.assertIn("ci-gate", self.job_needs(data, job))

    def test_release_workflows_default_contents_to_read_and_escalate_writes(self):
        for filename in ("release.yml", "snapshot.yml"):
            text = self.read(filename)
            with self.subTest(workflow=filename):
                self.assertRegex(text, r"(?m)^  contents: read\s*$")
                self.assertRegex(text, r"(?m)^      contents: write\s*$")

    def test_main_image_binary_version_matches_oci_label(self):
        text = self.read("docker-publish.yml")
        self.assertIn('VERSION="${{ github.ref_name }}"', text)
        self.assertIn('OCI_VERSION=${{ github.ref_name }}', text)

    def test_oci_version_is_preserved_in_image_publishers(self):
        for filename in ("release.yml", "snapshot.yml", "release-docker.yml", "docker-publish.yml"):
            with self.subTest(workflow=filename):
                self.assertIn("OCI_VERSION=", self.read(filename))


if __name__ == "__main__":
    unittest.main()
