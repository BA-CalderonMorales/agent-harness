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
PRIVATE_PATH = "/home/someone/clients/acme-7731/plan.txt"
RECORD = {"type": "record", "created_at": "0001-01-01T00:00:00Z", "record": {
    "v": 1, "goal": f"fix {SECRET}", "constraints": [f"Never touch {PRIVATE_PATH}"],
    "files": [{"path": PRIVATE_PATH, "op": "edit", "n": 2}],
    "cmds": [{"cmd": f"cat {PRIVATE_PATH} {SECRET}", "failed": True, "err": SECRET}],
    "todos": [{"text": SECRET, "status": "in_progress"}],
    "verified": [f"go test {SECRET}"], "pending": [SECRET], "denied": [f"Permission denied: {SECRET}"],
    "last_user": SECRET, "replied": False,
    "open": [{"id": "real-call-1", "tool": "bash", "arg": SECRET}]}}


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
    assert blocks[1]["input"]["nested"]["args"][1:] == [3, True, None], "non-string input values must survive"
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
         "input": {"file_path": SECRET, "files": {private_key: SECRET, "Notes For Q3": [SECRET]}}}]}}
    result = sanitize([HEADER, keyed])
    assert result.returncode == 0, result.stderr
    assert "customer-plan" not in result.stdout and "Notes For Q3" not in result.stdout, result.stdout
    assert SECRET not in result.stdout, result.stdout
    block = json.loads(result.stdout.splitlines()[1])["message"]["content"][0]["input"]
    assert block["file_path"] == "sanitized-file_path", block
    assert all(k.startswith("key-") for k in block["files"]), block

    # The record line: every string is replaced, structure and vocabularies stay.
    result = sanitize([HEADER, CALL, RESULT, RECORD, META])
    assert result.returncode == 0, result.stderr
    for leak in (SECRET, "someone", "acme", "plan.txt"):
        assert leak not in result.stdout, f"record leaked {leak!r}: {result.stdout}"
    kept = json.loads(result.stdout.splitlines()[3])
    assert kept["type"] == "record" and kept["record"]["v"] == 1
    body = kept["record"]
    assert body["files"][0]["op"] == "edit" and body["files"][0]["n"] == 2
    assert body["cmds"][0]["failed"] is True and body["todos"][0]["status"] == "in_progress"
    assert body["open"][0]["id"] == call_id and body["open"][0]["tool"] == "bash", "open calls must keep their call link"
    assert body["goal"].startswith("sanitized-goal-") and body["files"][0]["path"].startswith("sanitized/path-")

    def with_record(**changes):
        return [HEADER, {**RECORD, "record": {**RECORD["record"], **changes}}]

    expect_refused("record field", with_record(secret_notes=SECRET), "line 2.record.secret_notes")
    expect_refused("record file field", with_record(files=[{"path": "p", "op": "read", "n": 1, "owner": SECRET}]),
                   "line 2.record.files[0].owner")
    expect_refused("record op vocabulary", with_record(files=[{"path": "p", "op": SECRET, "n": 1}]),
                   "line 2.record.files[0].op")
    expect_refused("record status vocabulary", with_record(todos=[{"text": "t", "status": SECRET}]),
                   "line 2.record.todos[0].status")
    expect_refused("record wrong type", with_record(goal=["x"]), "line 2.record.goal")

    print("sanitize-session: placeholders, structure, call links, data-bearing keys, and refusal of unknown fields at every level passed")


if __name__ == "__main__":
    main()
