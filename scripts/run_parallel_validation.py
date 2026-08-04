#!/usr/bin/env python3
"""Run independent Go validation checks concurrently after shared CI setup."""

from collections.abc import Callable, Mapping
from concurrent.futures import ThreadPoolExecutor, as_completed
from contextlib import contextmanager
from dataclasses import dataclass
from pathlib import Path
import os
import subprocess
import time


ROOT = Path(__file__).resolve().parents[1]


@dataclass(frozen=True)
class CheckResult:
    name: str
    returncode: int
    output: str
    duration_seconds: float


def run_command(name: str, command: list[str], env: dict[str, str] | None = None) -> CheckResult:
    """Run one command and capture combined output for non-interleaved CI logs."""
    started = time.monotonic()
    try:
        process = subprocess.run(
            command,
            cwd=ROOT,
            env=env,
            check=False,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
        return CheckResult(
            name=name,
            returncode=process.returncode,
            output=process.stdout,
            duration_seconds=time.monotonic() - started,
        )
    except OSError as error:
        return CheckResult(
            name=name,
            returncode=127,
            output=f"Unable to execute {command[0]}: {error}\n",
            duration_seconds=time.monotonic() - started,
        )


def check_build() -> CheckResult:
    return run_command("build", ["go", "build", "./..."])


def check_quality() -> CheckResult:
    started = time.monotonic()
    vet = run_command("go vet", ["go", "vet", "./cmd/...", "./pkg/..."])
    formatting = run_command("gofmt", ["gofmt", "-l", "cmd", "pkg", "test"])

    output = f"[go vet]\n{vet.output}[gofmt]\n{formatting.output}"
    returncode = vet.returncode or formatting.returncode
    if formatting.returncode == 0 and formatting.output.strip():
        output += "Formatting check found files that require gofmt.\n"
        returncode = 1
    return CheckResult(
        name="static quality",
        returncode=returncode,
        output=output,
        duration_seconds=time.monotonic() - started,
    )


def check_unit() -> CheckResult:
    environment = os.environ.copy()
    environment["API_ENV"] = "unit_testing"
    return run_command(
        "unit tests",
        ["go", "test", "-vet=off", "-short", "./pkg/...", "./cmd/..."],
        env=environment,
    )


def execute_checks(
    checks: Mapping[str, Callable[[], CheckResult]],
) -> list[CheckResult]:
    """Execute all supplied checks concurrently and return completion-ordered results."""
    with ThreadPoolExecutor(max_workers=len(checks)) as executor:
        futures = [executor.submit(check) for check in checks.values()]
        return [future.result() for future in as_completed(futures)]


def report_results(results: list[CheckResult]) -> bool:
    """Emit grouped GitHub logs and return whether every check passed."""
    all_passed = True
    for result in results:
        print(f"::group::{result.name}")
        if result.output:
            print(result.output, end="" if result.output.endswith("\n") else "\n")
        print("::endgroup::")
        if result.returncode == 0:
            print(f"::notice::{result.name} passed in {result.duration_seconds:.1f}s")
        else:
            print(
                f"::error::{result.name} failed with exit code {result.returncode} "
                f"after {result.duration_seconds:.1f}s"
            )
            all_passed = False
    return all_passed


@contextmanager
def unit_test_password():
    """Create the ignored unit-test fixture only when one is not already present."""
    password_path = ROOT / "secrets" / "db.password"
    created = not password_path.exists()
    if created:
        password_path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        password_path.write_text("ci-test-password\n", encoding="utf-8")
        password_path.chmod(0o600)
    try:
        yield
    finally:
        if created:
            password_path.unlink(missing_ok=True)


def main() -> int:
    checks = {
        "build": check_build,
        "static quality": check_quality,
        "unit tests": check_unit,
    }
    with unit_test_password():
        results = execute_checks(checks)
    return 0 if report_results(results) else 1


if __name__ == "__main__":
    raise SystemExit(main())
