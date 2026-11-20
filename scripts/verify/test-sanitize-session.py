"""Tests for scripts/verify/sanitize-session.py: private content never reaches a fixture."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile

SCRIPT = Path(__file__).resolve().parent / "sanitize-session.py"
SECRET = "PRIVATE-VALUE-7731"

HEADER = {"type": "header", "id": "real-session-id", "created_at": "2026-10-27T08:22:25Z",
          "model": "qwen3.5-2b-FLM", "persona": "developer", "version": 2, "turns": 3}
CALL = {"type": "message", "created_at": "0001-01-01T00:00:00Z", "message": {
    "uuid": "real-uuid-1", "role": "assistant", "timestamp": "2026-10-27T08:23:00Z",
    "stop_reason": "tool_calls", "model": "qwen3.5-2b-FLM", "api_error": SECRET,
    "content": [{"text": SECRET},
                {"id": "real-call-1", "name": "bash",
                 "input": {"command": SECRET, "nested": {"args": [SECRET, 3, True, None]}}}]}}
RESULT = {"type": "message", "created_at": "0001-01-01T00:00:00Z", "message": {
    "uuid": "real-uuid-2", "role": "user", "timestamp": "2026-10-27T08:23:05Z",
    "content": [{"tool_use_id": "real-call-1", "content": SECRET, "is_error": True}]}}
META = {"type": "meta", "created_at": "0001-01-01T00:00:00Z",
        "meta": {"model": "qwen3.5-2b-FLM", "persona": "developer", "turns": 3}}


def sanitize(records):
    with tempfile.TemporaryDirectory(prefix="ah-sanitize-test-") as directory:
        source = Path(directory) / "session.jsonl"
        source.write_text("".join(json.dumps(r) + "\n" for r in records))
        return subprocess.run([sys.executable, "-I", str(SCRIPT), str(source), "fixture"],
                              capture_output=True, text=True)


def expect_refused(label, records, field):
    result = sanitize(records)
    assert result.returncode != 0, f"{label}: accepted an unreviewed field"
    assert field in result.stderr, f"{label}: refusal does not name {field!r}: {result.stderr}"
    assert result.stdout == "", f"{label}: a refused run still printed part of a fixture"
    assert SECRET not in result.stdout + result.stderr, f"{label}: the refusal leaked the value"


def main():
    result = sanitize([HEADER, CALL, RESULT, META])
    assert result.returncode == 0, result.stderr
    assert SECRET not in result.stdout, "private content reached the fixture"
    header, call, answer, meta = (json.loads(line) for line in result.stdout.splitlines())

    assert header["id"] == "session-fixture" and header["version"] == 2 and header["turns"] == 3
    assert header["model"] == "qwen3.5-2b-FLM" and meta["meta"]["turns"] == 3
    blocks = call["message"]["content"]
    assert blocks[1]["name"] == "bash", "tool names are structural and must survive"
    nested = blocks[1]["input"]["nested"]
    assert list(nested) and all(k.startswith("key-") for k in nested), "nested keys are data and must be replaced"
    assert list(nested.values())[0][1:] == [3, True, None], "non-string input values must survive"
    call_id = blocks[1]["id"]
    assert call_id.startswith("toolu-") and answer["message"]["content"][0]["tool_use_id"] == call_id, \
        "a result must still point at the call it answers"
    assert answer["message"]["content"][0]["is_error"] is True
    assert call["message"]["uuid"].startswith("msg-") and answer["message"]["uuid"].startswith("msg-")
    assert call["message"]["uuid"] != answer["message"]["uuid"]

    # Anything the schema does not name stops the run, at every level and
    # whether it holds a string or a container.
    expect_refused("record string", [dict(HEADER, extra=SECRET)], "line 1.extra")
    expect_refused("record container", [dict(HEADER, metadata={"api_key": SECRET})], "line 1.metadata")
    expect_refused("message container", [HEADER, {**CALL, "message": {**CALL["message"], "usage": {"note": SECRET}}}],
                   "line 2.message.usage")
    expect_refused("meta string", [HEADER, {**META, "meta": {**META["meta"], "workspace": SECRET}}],
                   "line 2.meta.workspace")
    expect_refused("block field", [HEADER, {**RESULT, "message": {**RESULT["message"], "content": [
        {"tool_use_id": "real-call-1", "content": "x", "attachments": [SECRET]}]}}],
                   "line 2.message.content[0].attachments")
    expect_refused("wrong type", [HEADER, {**META, "meta": {**META["meta"], "turns": SECRET}}],
                   "line 2.meta.turns")

    # A key inside tool input can itself be data, such as a file name. It
    # is replaced, never repeated in a placeholder; parameter names stay.
    private_key = "/private/customer-plan-7731.txt"
    keyed = {**CALL, "message": {**CALL["message"], "content": [
        {"id": "real-call-9", "name": "write",
         "input": {"file_path": SECRET, "files": {private_key: SECRET, "Notes For Q3": [SECRET],
                                                   "customer_plan_7731": SECRET}}}]}}
    result = sanitize([HEADER, keyed])
    assert result.returncode == 0, result.stderr
    assert "customer-plan" not in result.stdout and "Notes For Q3" not in result.stdout, result.stdout
    # A nested key shaped like an identifier is still data, not a parameter.
    assert "customer_plan" not in result.stdout, result.stdout
    assert SECRET not in result.stdout, result.stdout
    block = json.loads(result.stdout.splitlines()[1])["message"]["content"][0]["input"]
    assert block["file_path"] == "sanitized-file_path", block
    assert all(k.startswith("key-") for k in block["files"]), block

    print("sanitize-session: placeholders, structure, call links, data-bearing keys, and refusal of unknown fields at every level passed")


if __name__ == "__main__":
    main()
