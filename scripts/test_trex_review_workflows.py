#!/usr/bin/env python3
"""Offline policy tests for TRex pull request workflow trust boundaries."""

from pathlib import Path
import re
import unittest


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

    violations.extend(validate_action_pins(workflow))
    violations.extend(validate_expression_operators(workflow))
    return violations


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

    def test_rejects_missing_ready_for_review_trigger(self) -> None:
        unsafe = self.ci.replace(", ready_for_review", "")
        self.assertPolicyRejects(validate_ci_workflow, unsafe, "ready_for_review")

    def test_rejects_mutable_action_reference(self) -> None:
        unsafe = re.sub(r"(uses:\s*actions/checkout@)[0-9a-f]{40}", r"\1v5", self.ci)
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
