from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import pytest

import security_review as sr


class ExtractJsonTest(unittest.TestCase):
    def test_plain(self) -> None:
        assert sr.extract_json('{"a": 1}') == {"a": 1}

    def test_fenced(self) -> None:
        assert sr.extract_json('```json\n{"a": 1}\n```') == {"a": 1}

    def test_wrapped_in_prose(self) -> None:
        assert sr.extract_json('Here you go:\n{"a": 1}\nHope it helps.') == {"a": 1}


class ValidFindingTest(unittest.TestCase):
    def valid(self) -> dict[str, object]:
        return {
            "rule_id": "sql-injection",
            "title": "SQL injection",
            "severity": "HIGH",
            "confidence": 0.9,
            "file": "src/app.py",
            "line": 12,
            "description": "d",
            "exploit_scenario": "e",
            "recommendation": "r",
        }

    def test_valid(self) -> None:
        assert sr.valid_finding(self.valid())

    def test_optional_fields(self) -> None:
        finding = self.valid()
        finding["end_line"] = 14
        finding["cwe"] = "CWE-89"
        assert sr.valid_finding(finding)

    def test_missing_field(self) -> None:
        finding = self.valid()
        del finding["file"]
        assert not sr.valid_finding(finding)

    def test_bad_severity(self) -> None:
        finding = self.valid()
        finding["severity"] = "EXTREME"
        assert not sr.valid_finding(finding)

    def test_bad_rule_id(self) -> None:
        finding = self.valid()
        finding["rule_id"] = "SQL Injection"
        assert not sr.valid_finding(finding)

    def test_bad_confidence(self) -> None:
        finding = self.valid()
        finding["confidence"] = 2
        assert not sr.valid_finding(finding)

    def test_bool_is_not_a_number(self) -> None:
        finding = self.valid()
        finding["confidence"] = True
        assert not sr.valid_finding(finding)

    def test_non_dict(self) -> None:
        assert not sr.valid_finding("nope")


class NormalizeTest(unittest.TestCase):
    def events(self, text: str) -> str:
        return "\n".join(
            json.dumps(event)
            for event in [
                {
                    "type": "step_start",
                    "timestamp": 1000,
                    "sessionID": "ses_1",
                    "part": {"type": "step-start"},
                },
                {
                    "type": "text",
                    "timestamp": 2000,
                    "sessionID": "ses_1",
                    "part": {"type": "text", "text": text},
                },
                {
                    "type": "step_finish",
                    "timestamp": 3000,
                    "sessionID": "ses_1",
                    "part": {
                        "type": "step-finish",
                        "tokens": {
                            "input": 100,
                            "output": 20,
                            "reasoning": 0,
                            "cache": {"read": 5, "write": 3},
                        },
                        "cost": 0.001,
                        "reason": "stop",
                    },
                },
            ]
        )

    def normalize(
        self, text: str, model: str = "opencode/deepseek-v4.1-flash"
    ) -> dict[str, object]:
        with tempfile.TemporaryDirectory() as tmp:
            events = Path(tmp) / "opencode.jsonl"
            events.write_text(self.events(text))
            out = Path(tmp) / "result.json"
            sr.normalize(argparse.Namespace(events=str(events), model=model, out=str(out)))
            return json.loads(out.read_text())

    def test_roundtrip(self) -> None:
        finding = {
            "rule_id": "sql-injection",
            "title": "SQL injection",
            "severity": "HIGH",
            "confidence": 0.9,
            "file": "src/app.py",
            "line": 12,
            "description": "d",
            "exploit_scenario": "e",
            "recommendation": "r",
        }
        result = self.normalize(json.dumps({"findings": [finding], "resolved_alerts": [1, 2]}))
        assert not result["is_error"]
        assert result["structured_output"] == {"findings": [finding], "resolved_alerts": [1, 2]}
        assert result["usage"]["input_tokens"] == 100  # ty: ignore[not-subscriptable]
        assert result["usage"]["output_tokens"] == 20  # ty: ignore[not-subscriptable]
        assert result["usage"]["cache_read_input_tokens"] == 5  # ty: ignore[not-subscriptable]
        assert result["usage"]["cache_creation_input_tokens"] == 3  # ty: ignore[not-subscriptable]
        assert result["num_turns"] == 1
        assert result["duration_ms"] == 2000
        assert result["total_cost_usd"] == 0.001
        assert result["modelUsage"]["opencode/deepseek-v4.1-flash"]["outputTokens"] == 20  # ty: ignore[not-subscriptable]

    def test_fenced_output(self) -> None:
        result = self.normalize('```json\n{"findings": [], "resolved_alerts": []}\n```')
        assert result["structured_output"] == {"findings": [], "resolved_alerts": []}

    def test_malformed_finding_fails_instead_of_silently_dropping_it(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            events = Path(tmp) / "opencode.jsonl"
            events.write_text(
                self.events(json.dumps({"findings": [{"severity": "HIGH"}], "resolved_alerts": []}))
            )
            out = Path(tmp) / "result.json"
            with pytest.raises(SystemExit, match="malformed finding"):
                sr.normalize(
                    argparse.Namespace(
                        events=str(events), model="opencode/deepseek-v4.1-flash", out=str(out)
                    )
                )
            assert not out.exists()

    def test_non_integer_alert_number_fails(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            events = Path(tmp) / "opencode.jsonl"
            events.write_text(self.events(json.dumps({"findings": [], "resolved_alerts": ["1"]})))
            out = Path(tmp) / "result.json"
            with pytest.raises(SystemExit, match="non-integer alert number"):
                sr.normalize(
                    argparse.Namespace(
                        events=str(events), model="opencode/deepseek-v4.1-flash", out=str(out)
                    )
                )
            assert not out.exists()

    def test_error_event_fails(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            events = Path(tmp) / "opencode.jsonl"
            events.write_text(
                json.dumps({"type": "error", "error": {"name": "AuthenticationError"}})
            )
            out = Path(tmp) / "result.json"
            with pytest.raises(SystemExit):
                sr.normalize(argparse.Namespace(events=str(events), model="m", out=str(out)))


class SecretFilterTest(unittest.TestCase):
    def test_claude_credential_shape_matches(self) -> None:
        assert sr.SECRET.search("sk-ant-oat01-deadbeefdeadbeef") is not None

    def test_opencode_credential_shape_matches(self) -> None:
        assert sr.SECRET.search("oc_sk_deadbeefdeadbeef_deadbeef") is not None

    def test_plain_text_does_not_match(self) -> None:
        assert sr.SECRET.search("no credentials here") is None

    def refuses(self, secret: str) -> None:
        finding = {
            "rule_id": "leaked-credential",
            "title": "Leaked credential",
            "severity": "HIGH",
            "confidence": 0.9,
            "file": "src/app.py",
            "line": 1,
            "description": f"a hardcoded key: {secret}",
            "exploit_scenario": "e",
            "recommendation": "r",
        }
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "src").mkdir()
            (root / "src" / "app.py").write_text("x = 1")
            (root / "result.json").write_text(
                json.dumps({"structured_output": {"findings": [finding], "resolved_alerts": []}})
            )
            (root / "all.json").write_text("[]")
            (root / "shown.json").write_text("[]")
            (root / "deleted.txt").write_text("")
            old = os.getcwd()
            os.chdir(tmp)
            try:
                with pytest.raises(SystemExit):
                    sr.sarif(
                        argparse.Namespace(
                            result=str(root / "result.json"),
                            alerts=str(root / "all.json"),
                            shown=str(root / "shown.json"),
                            deleted=str(root / "deleted.txt"),
                            tool="agentic-security-review-opencode",
                            out=str(root / "out.sarif"),
                        )
                    )
            finally:
                os.chdir(old)

    def test_sarif_refuses_claude_credential(self) -> None:
        self.refuses("sk-ant-oat01-deadbeefdeadbeef")

    def test_sarif_refuses_opencode_credential(self) -> None:
        self.refuses("oc_sk_deadbeefdeadbeef_deadbeef")


class ToolIsolationTest(unittest.TestCase):
    def test_fingerprints_differ_per_tool(self) -> None:
        claude = sr.fingerprint(
            "agentic-security-review-claude", "sql-injection", "src/app.py", "SQLi"
        )
        opencode = sr.fingerprint(
            "agentic-security-review-opencode", "sql-injection", "src/app.py", "SQLi"
        )
        assert claude != opencode

    def test_result_keys_differ_per_tool(self) -> None:
        location = {"path": "src/app.py", "start_line": 1}
        claude = sr.result("agentic-security-review-claude", "r", "HIGH", location, "t")
        opencode = sr.result("agentic-security-review-opencode", "r", "HIGH", location, "t")
        assert list(claude["partialFingerprints"]) == ["agentic-security-review-claude/v1"]
        assert list(opencode["partialFingerprints"]) == ["agentic-security-review-opencode/v1"]
        assert (
            claude["partialFingerprints"]["agentic-security-review-claude/v1"]
            != opencode["partialFingerprints"]["agentic-security-review-opencode/v1"]
        )

    def test_sarif_driver_name_per_tool(self) -> None:
        finding = {
            "rule_id": "sql-injection",
            "title": "SQL injection",
            "severity": "HIGH",
            "confidence": 0.9,
            "file": "src/app.py",
            "line": 1,
            "description": "d",
            "exploit_scenario": "e",
            "recommendation": "r",
        }
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "src").mkdir()
            (root / "src" / "app.py").write_text("x = 1")
            (root / "result.json").write_text(
                json.dumps({"structured_output": {"findings": [finding], "resolved_alerts": []}})
            )
            (root / "all.json").write_text("[]")
            (root / "shown.json").write_text("[]")
            (root / "deleted.txt").write_text("")
            old = os.getcwd()
            os.chdir(tmp)
            try:
                for tool in ("agentic-security-review-claude", "agentic-security-review-opencode"):
                    sr.sarif(
                        argparse.Namespace(
                            result=str(root / "result.json"),
                            alerts=str(root / "all.json"),
                            shown=str(root / "shown.json"),
                            deleted=str(root / "deleted.txt"),
                            tool=tool,
                            out=str(root / f"{tool}.sarif"),
                        )
                    )
            finally:
                os.chdir(old)
            claude = json.loads((root / "agentic-security-review-claude.sarif").read_text())
            opencode = json.loads((root / "agentic-security-review-opencode.sarif").read_text())
            assert claude["runs"][0]["tool"]["driver"]["name"] == "agentic-security-review-claude"
            assert (
                opencode["runs"][0]["tool"]["driver"]["name"] == "agentic-security-review-opencode"
            )
            assert list(claude["runs"][0]["results"][0]["partialFingerprints"]) == [
                "agentic-security-review-claude/v1"
            ]
            assert list(opencode["runs"][0]["results"][0]["partialFingerprints"]) == [
                "agentic-security-review-opencode/v1"
            ]


if __name__ == "__main__":
    unittest.main()
