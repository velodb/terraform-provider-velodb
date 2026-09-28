#!/usr/bin/env python3
"""Exercise the actual module guards with Terraform's built-in provider only."""
import json
from pathlib import Path
import re
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]


def run(directory, *args, success=True):
    result = subprocess.run(["terraform", *args, "-no-color"], cwd=directory,
                            capture_output=True, text=True)
    if (result.returncode == 0) != success:
        raise AssertionError(result.stdout + result.stderr)
    return result.stdout + result.stderr


for module in ("aws_byoc", "aws_byoc_existing"):
    source = (ROOT / "modules" / module / "immutable.tf").read_text()
    fields = re.findall(r"jsonencode\(var\.(\w+)\)", source)
    baseline = {}
    for field in fields:
        if field.startswith("create_"):
            value = False
        elif field.endswith("_arn") and "kms" in field:
            value = None
        elif field == "zones":
            value = ["us-east-1a"]
        elif field in ("subnet_cidrs", "subnet_ids_by_zone"):
            value = {"us-east-1a": "original"}
        else:
            value = "original"
        baseline[field] = value
    with tempfile.TemporaryDirectory(prefix="velodb-immutable-") as tmp:
        directory = Path(tmp)
        (directory / "main.tf").write_text(source + "\n" + "\n".join(
            f'variable "{field}" {{}}' for field in fields))
        variables = directory / "terraform.tfvars.json"
        variables.write_text(json.dumps(baseline))
        run(directory, "init", "-backend=false")
        run(directory, "apply", "-auto-approve")
        run(directory, "plan", "-detailed-exitcode")
        for field, original in baseline.items():
            changed = dict(baseline)
            if isinstance(original, bool):
                replacement = True
            elif isinstance(original, list):
                replacement = ["us-east-1b"]
            elif isinstance(original, dict):
                replacement = {"us-east-1b": "changed"}
            else:
                replacement = "changed"
            changed[field] = replacement
            variables.write_text(json.dumps(changed))
            output = run(directory, "plan", success=False)
            assert f"{field} cannot be changed" in output, output
            print(f"PASS {module}: rejects {field}")
        variables.write_text(json.dumps(baseline))
        run(directory, "plan", "-detailed-exitcode")
        run(directory, "destroy", "-auto-approve")
        # Verify disabling previously enabled encryption as well as enabling it.
        for field in fields:
            if field.startswith("create_"):
                baseline[field] = True
        variables.write_text(json.dumps(baseline))
        run(directory, "apply", "-auto-approve")
        for field in fields:
            if field.startswith("create_"):
                variables.write_text(json.dumps(dict(baseline, **{field: False})))
                output = run(directory, "plan", success=False)
                assert f"{field} cannot be changed" in output, output
                print(f"PASS {module}: rejects disabling {field}")
        variables.write_text(json.dumps(baseline))
        run(directory, "destroy", "-auto-approve")
