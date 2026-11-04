"""Validate untrusted observations before terminal automation consumes them."""
import json

MAX_RECORD = 1024 * 1024


def parse_event(line):
    if len(line) > MAX_RECORD:
        raise ValueError("event record exceeds capture budget")
    event = json.loads(line)
    if not isinstance(event, dict) or event.get("schema_version") != 1:
        raise ValueError("invalid event schema")
    if type(event.get("turn")) is not int or event["turn"] < 0:
        raise ValueError("invalid event turn")
    fields = {
        "window": {"width": int, "height": int}, "start": {},
        "submit": {"text": str}, "text": {"text": str},
        "complete": {"text": str}, "error": {"error": str},
        "tool_start": {"id": str, "tool": str, "input": dict},
        "tool_done": {"id": str, "success": bool, "output": str},
        "approval": {"id": str, "tool": str, "command": str, "input": dict},
        "cancel": {}, "question": {},
    }
    kind = event.get("type")
    if not isinstance(kind, str) or kind not in fields:
        raise ValueError("unknown event type")
    for name, expected in fields[kind].items():
        if type(event.get(name)) is not expected:
            raise ValueError(f"invalid {kind} event field: {name}")
    return event
