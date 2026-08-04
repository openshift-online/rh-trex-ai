#!/usr/bin/env python3
"""Offline policy tests for TRex pull request workflow trust boundaries."""

from contextlib import redirect_stdout
import io
from pathlib import Path
import re
import threading
import unittest
from unittest import mock

from scripts import run_parallel_validation


ROOT = Path(__file__).resolve().parents[1]
CI_WORKFLOW = ROOT / ".github" / "workflows" / "trex-pr-ci.yml"
COMMENT_WORKFLOW = ROOT / ".github" / "workflows" / "trex-auto-review.yml"
GITIGNORE = ROOT / ".gitignore"

ACTION_REFERENCE = re.compile(r"^\s*uses:\s*([^\s#]+)", re.MULTILINE)
EXPRESSION = re.compile(r"\$\{\{(.*?)}}", re.DOTALL)


def validate_ci_workflow(workflow: str) -> list[str]:
    """Return policy violations in the untrusted pull request workflow."""
    violations = []

    if not re.search(r"^\s{2}pull_request:\s*$", workflow, re.MULTILINE):
        violations.append("CI must use the pull_request event")
    if "pull_request_target" in workflow:
        violations.append("CI must not use pull_request_target")

    for event_type in ("opened", "synchronize", "reopened", "ready_for_review"):
        if event_type not in workflow:
            violations.append(f"CI must run for {event_type} pull requests")

    permissions = re.search(
        r"^permissions:\s*\n(?P<body>(?:^  [^\n]+\n?)+)", workflow, re.MULTILINE
    )
    if permissions is None or permissions.group("body").strip() != "contents: read":
        violations.append("CI permissions must be limited to contents: read")
    if re.search(r"^\s+[^#\n]+:\s*write\s*$", workflow, re.MULTILINE):
        violations.append("CI must not have write permissions")

    if "github.event.pull_request.draft == false" not in workflow:
        violations.append("CI must skip draft pull requests")
    if "persist-credentials: false" not in workflow:
        violations.append("CI checkout must not persist credentials")

    violations.extend(validate_ci_job_graph(workflow))
    violations.extend(validate_action_pins(workflow))
    violations.extend(validate_expression_operators(workflow))
    return violations


def validate_ci_job_graph(workflow: str) -> list[str]:
    """Validate the single shared-setup job and in-runner concurrency contract."""
    violations = []
    jobs = workflow_job_blocks(workflow)

    if set(jobs) != {"validate"}:
        violations.append("CI must use exactly one shared-setup validate job")
    validate = jobs.get("validate")
    if validate is None:
        return violations
    if "name: validate" not in validate:
        violations.append("CI job must retain the validate check name")
    if re.search(r"^    needs:", validate, re.MULTILINE):
        violations.append("CI validate job must not depend on another setup job")
    if "if: github.event.pull_request.draft == false" not in validate:
        violations.append("CI validate job must skip draft pull requests")
    if "continue-on-error:" in validate:
        violations.append("CI validate checks must propagate failures")

    required_commands = (
        "python3 -m unittest scripts/test_trex_review_workflows.py",
        "python3 scripts/run_parallel_validation.py",
    )
    for command in required_commands:
        if command not in validate:
            violations.append(f"CI validate job is missing required command: {command}")

    shared_setup_counts = {
        "checkout": workflow.count("actions/checkout@"),
        "Go setup": workflow.count("actions/setup-go@"),
        "dependency download": workflow.count("go mod download"),
        "dependency verification": workflow.count("go mod verify"),
    }
    expected_counts = {
        "checkout": 1,
        "Go setup": 1,
        "dependency download": 1,
        "dependency verification": 1,
    }
    for operation, expected in expected_counts.items():
        if shared_setup_counts[operation] != expected:
            violations.append(
                f"CI must perform {operation} exactly {expected} time(s)"
            )

    return violations


def workflow_job_blocks(workflow: str) -> dict[str, str]:
    """Extract top-level job blocks without requiring a YAML dependency."""
    _, separator, jobs_section = workflow.partition("\njobs:\n")
    if not separator:
        return {}
    headers = list(
        re.finditer(r"^  ([a-z][a-z0-9_-]*):\s*$", jobs_section, re.MULTILINE)
    )
    blocks = {}
    for index, header in enumerate(headers):
        end = headers[index + 1].start() if index + 1 < len(headers) else len(jobs_section)
        blocks[header.group(1)] = jobs_section[header.start():end]
    return blocks


def validate_comment_workflow(workflow: str) -> list[str]:
    """Return policy violations in the privileged commenting workflow."""
    violations = []

    if not re.search(r"^\s{2}workflow_run:\s*$", workflow, re.MULTILINE):
        violations.append("commenting must use the workflow_run event")
    if "workflows: [TRex PR CI]" not in workflow or "types: [completed]" not in workflow:
        violations.append("commenting must consume completed TRex PR CI runs")
    if "pull_request_target" in workflow:
        violations.append("commenting must not use pull_request_target")

    required_permissions = {
        "actions: read",
        "contents: read",
        "issues: write",
        "pull-requests: read",
    }
    permissions = re.search(
        r"^permissions:\s*\n(?P<body>(?:^  [^\n]+\n?)+)", workflow, re.MULTILINE
    )
    actual_permissions = (
        {line.strip() for line in permissions.group("body").splitlines()}
        if permissions
        else set()
    )
    if actual_permissions != required_permissions:
        violations.append("commenting permissions do not match the approved minimum set")

    prohibited = {
        "pull request checkout": r"actions/checkout",
        "shell execution": r"^\s*-?\s*run:\s*",
        "artifact download": r"actions/download-artifact|downloadArtifact",
        "artifact upload": r"actions/upload-artifact|uploadArtifact",
        "cache use": r"actions/cache|restore-cache|save-cache",
        "dynamic code or file import": (
            r"\brequire\s*\(|\bimport\s*\(|\beval\s*\(|"
            r"\bnew\s+Function\s*\(|\bchild_process\b|\bfs\."
        ),
    }
    for operation, pattern in prohibited.items():
        if re.search(pattern, workflow, re.MULTILINE | re.IGNORECASE):
            violations.append(f"commenting must not perform {operation}")

    required_api_operations = (
        "listPullRequestsAssociatedWithCommit",
        "pulls.listFiles",
        "listJobsForWorkflowRun",
        "issues.updateComment",
        "issues.createComment",
        "<!-- trex-auto-review -->",
    )
    for operation in required_api_operations:
        if operation not in workflow:
            violations.append(f"commenting is missing API operation or marker: {operation}")

    violations.extend(validate_action_pins(workflow))
    violations.extend(validate_expression_operators(workflow))
    return violations


def validate_action_pins(workflow: str) -> list[str]:
    violations = []
    references = ACTION_REFERENCE.findall(workflow)
    if not references:
        violations.append("workflow must use at least one action")
    for reference in references:
        if reference.startswith("./"):
            continue
        _, separator, revision = reference.rpartition("@")
        if not separator or re.fullmatch(r"[0-9a-f]{40}", revision) is None:
            violations.append(f"action reference is not an immutable SHA: {reference}")
    return violations


def validate_expression_operators(workflow: str) -> list[str]:
    violations = []
    expressions = EXPRESSION.findall(workflow)
    expressions.extend(
        re.findall(r"^\s*if:\s*(.*?)\s*$", workflow, re.MULTILINE)
    )
    for expression in expressions:
        if "===" in expression or "!==" in expression:
            violations.append("GitHub expressions must use == or !=, not JavaScript operators")
    return violations


class ReviewWorkflowPolicyTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.ci = CI_WORKFLOW.read_text(encoding="utf-8")
        cls.commenter = COMMENT_WORKFLOW.read_text(encoding="utf-8")

    def test_repository_workflows_satisfy_policy(self) -> None:
        violations = validate_ci_workflow(self.ci) + validate_comment_workflow(self.commenter)
        self.assertEqual([], violations, "\n".join(violations))

    def test_python_caches_are_ignored(self) -> None:
        ignore_rules = GITIGNORE.read_text(encoding="utf-8").splitlines()
        self.assertIn("__pycache__/", ignore_rules)
        self.assertIn("*.py[cod]", ignore_rules)
        self.assertIn(".pytest_cache/", ignore_rules)

    def test_rejects_privileged_checkout(self) -> None:
        unsafe = self.commenter + "\n      - uses: actions/checkout@" + "0" * 40
        self.assertPolicyRejects(validate_comment_workflow, unsafe, "checkout")

    def test_rejects_privileged_shell_execution(self) -> None:
        unsafe = self.commenter + "\n      - run: ./fork-controlled-script.sh\n"
        self.assertPolicyRejects(validate_comment_workflow, unsafe, "shell execution")

    def test_rejects_privileged_artifact_or_cache_use(self) -> None:
        for action in ("actions/download-artifact", "actions/upload-artifact", "actions/cache"):
            with self.subTest(action=action):
                unsafe = self.commenter + f"\n      - uses: {action}@" + "0" * 40
                self.assertTrue(validate_comment_workflow(unsafe))

    def test_rejects_privileged_code_or_file_import(self) -> None:
        for expression in ("require('fs')", "eval(untrustedPatch)", "import(untrustedPath)"):
            with self.subTest(expression=expression):
                unsafe = self.commenter + f"\n          script: {expression}\n"
                self.assertPolicyRejects(
                    validate_comment_workflow, unsafe, "dynamic code or file import"
                )

    def test_rejects_pull_request_target(self) -> None:
        unsafe = self.commenter + "\n  pull_request_target:\n"
        self.assertPolicyRejects(validate_comment_workflow, unsafe, "pull_request_target")

    def test_rejects_ci_write_permission(self) -> None:
        unsafe = self.ci.replace("  contents: read", "  contents: write")
        self.assertPolicyRejects(validate_ci_workflow, unsafe, "write permissions")

    def test_rejects_duplicate_setup_job(self) -> None:
        unsafe = self.ci + "\n  duplicate:\n    runs-on: ubuntu-latest\n"
        self.assertPolicyRejects(validate_ci_workflow, unsafe, "exactly one")

    def test_rejects_missing_parallel_runner(self) -> None:
        unsafe = self.ci.replace(
            "python3 scripts/run_parallel_validation.py",
            "echo 'parallel validation disabled'",
        )
        self.assertPolicyRejects(validate_ci_workflow, unsafe, "missing required command")

    def test_rejects_duplicate_go_setup(self) -> None:
        setup = "actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff"
        unsafe = self.ci + f"\n      - uses: {setup}\n"
        self.assertPolicyRejects(validate_ci_workflow, unsafe, "Go setup exactly 1")

    def test_rejects_ignored_validation_failure(self) -> None:
        unsafe = self.ci + "\n        continue-on-error: true\n"
        self.assertPolicyRejects(validate_ci_workflow, unsafe, "propagate failures")

    def test_validation_runner_executes_checks_concurrently(self) -> None:
        barrier = threading.Barrier(3)

        def check(name: str):
            def run():
                barrier.wait(timeout=2)
                return run_parallel_validation.CheckResult(name, 0, "", 0.0)

            return run

        checks = {name: check(name) for name in ("build", "quality", "unit")}
        results = run_parallel_validation.execute_checks(checks)
        self.assertEqual(set(checks), {result.name for result in results})

    def test_validation_runner_reports_failures(self) -> None:
        failure = run_parallel_validation.CheckResult("build", 2, "build failed\n", 0.1)
        output = io.StringIO()
        with redirect_stdout(output):
            passed = run_parallel_validation.report_results([failure])
        self.assertFalse(passed)
        self.assertIn("::error::build failed with exit code 2", output.getvalue())

    def test_validation_runner_does_not_repeat_vet_during_unit_tests(self) -> None:
        result = run_parallel_validation.CheckResult("unit tests", 0, "", 0.0)
        with mock.patch.object(
            run_parallel_validation, "run_command", return_value=result
        ) as run_command:
            run_parallel_validation.check_unit()
        self.assertIn("-vet=off", run_command.call_args.args[1])

    def test_rejects_missing_ready_for_review_trigger(self) -> None:
        unsafe = self.ci.replace(", ready_for_review", "")
        self.assertPolicyRejects(validate_ci_workflow, unsafe, "ready_for_review")

    def test_rejects_mutable_action_reference(self) -> None:
        unsafe = re.sub(r"(uses:\s*actions/checkout@)[0-9a-f]{40}", r"\1v4", self.ci)
        self.assertPolicyRejects(validate_ci_workflow, unsafe, "immutable SHA")

    def test_rejects_javascript_operator_in_github_expression(self) -> None:
        unsafe = self.ci.replace("draft == false", "draft === false")
        self.assertPolicyRejects(validate_ci_workflow, unsafe, "JavaScript operators")

    def assertPolicyRejects(self, validator, workflow: str, expected: str) -> None:
        violations = validator(workflow)
        self.assertTrue(
            any(expected in violation for violation in violations),
            f"expected a violation containing {expected!r}; got {violations!r}",
        )


if __name__ == "__main__":
    unittest.main()
