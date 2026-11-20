"""Turn a real agent-harness session file into a shareable test fixture.

Usage: python3 scripts/verify/sanitize-session.py <session.jsonl> <fixture-name> > fixture.jsonl

The fixture keeps everything the loader depends on: record kinds and their
order, header version and fields, meta fields, message roles, timestamps,
content block shapes, tool names, error flags, and which tool result answers
which tool call. It replaces everything a person wrote or a tool returned:
text, tool inputs, tool output, and every ID. Unknown string fields stop the
run, so a new field is reviewed before it can leak into a fixture.
"""
import json
import sys

KEEP_STRINGS = {"type", "role", "created_at", "timestamp", "model", "persona", "name", "stop_reason"}


class Sanitizer:
    def __init__(self, fixture):
        self.fixture = fixture
        self.ids = {}
        self.texts = 0

    def fake_id(self, real, prefix):
        if real not in self.ids:
            self.ids[real] = f"{prefix}-{len(self.ids) + 1:04d}"
        return self.ids[real]

    def text(self, original):
        # Keep a similar length, capped, so size-sensitive paths still see
        # long and short content without the fixture growing large.
        self.texts += 1
        unit = f"sanitized text {self.texts:04d} "
        length = max(len(unit), min(len(original), 1200))
        return (unit * (length // len(unit) + 1))[:length]

    def value(self, key, value):
        if isinstance(value, str):
            return f"sanitized-{key}"
        if isinstance(value, dict):
            return {k: self.value(k, v) for k, v in value.items()}
        if isinstance(value, list):
            return [self.value(key, v) for v in value]
        return value

    def block(self, block):
        out = {}
        for key, value in block.items():
            if key == "text":
                out[key] = self.text(value)
            elif key == "thinking":
                out[key] = self.text(value)
            elif key == "signature":
                out[key] = "sanitized-signature"
            elif key == "id":
                out[key] = self.fake_id(value, "toolu")
            elif key == "tool_use_id":
                out[key] = self.fake_id(value, "toolu")
            elif key == "input":
                out[key] = self.value("input", value)
            elif key == "content":
                out[key] = self.text(value) if isinstance(value, str) else self.value("content", value)
            elif key in ("name", "is_error"):
                out[key] = value
            else:
                raise SystemExit(f"unknown content block field {key!r}; review it before sanitizing")
        return out

    def message(self, message):
        out = {}
        for key, value in message.items():
            if key == "uuid":
                out[key] = self.fake_id(value, "msg")
            elif key == "content":
                out[key] = [self.block(b) for b in value or []]
            elif key in KEEP_STRINGS or not isinstance(value, str):
                out[key] = value
            else:
                raise SystemExit(f"unknown message field {key!r}; review it before sanitizing")
        return out

    def record(self, record):
        out = {}
        for key, value in record.items():
            if key == "id":
                out[key] = f"session-{self.fixture}"
            elif key == "message":
                out[key] = self.message(value)
            elif key == "meta":
                out[key] = {k: v for k, v in value.items() if not isinstance(v, str) or k in KEEP_STRINGS}
            elif key in KEEP_STRINGS or not isinstance(value, str):
                out[key] = value
            else:
                raise SystemExit(f"unknown record field {key!r}; review it before sanitizing")
        return out


def main():
    if len(sys.argv) != 3:
        raise SystemExit(__doc__)
    path, fixture = sys.argv[1], sys.argv[2]
    sanitizer = Sanitizer(fixture)
    with open(path) as handle:
        for line in handle:
            if line.strip():
                print(json.dumps(sanitizer.record(json.loads(line)), separators=(",", ":")))


if __name__ == "__main__":
    main()
