"""Turn a real agent-harness session file into a shareable test fixture.

Usage: python3 scripts/verify/sanitize-session.py <session.jsonl> <fixture-name> > fixture.jsonl

The fixture keeps everything the loader depends on: record kinds and their
order, header version and fields, meta fields, message roles, timestamps,
content block shapes, tool names, error flags, and which tool result answers
which call. It replaces everything a person wrote or a tool returned: text,
tool inputs, tool output, provider errors, and every ID.

Every level has an explicit schema. A field the schema does not name, or a
value of an unexpected type, stops the run at any depth, so a new field is
reviewed before it can reach a fixture. Tool input is the one free-form
object; every string inside it is replaced, however deeply nested.
"""
import json
import sys


class Unreviewed(Exception):
    """A field or value the schema does not cover."""


def expect(value, kinds, where):
    if not isinstance(value, kinds) or isinstance(value, bool) and bool not in kinds:
        raise Unreviewed(f"{where}: unexpected {type(value).__name__} value")
    return value


def keep_str(value, where, _):
    return expect(value, (str,), where)


def keep_int(value, where, _):
    return expect(value, (int,), where)


def keep_bool(value, where, _):
    return expect(value, (bool,), where)


class Sanitizer:
    def __init__(self, fixture):
        self.fixture = fixture
        self.ids = {}
        self.texts = 0

    def fake_id(self, real, prefix):
        if real not in self.ids:
            self.ids[real] = f"{prefix}-{len(self.ids) + 1:04d}"
        return self.ids[real]

    def text(self, value, where, _=None):
        expect(value, (str,), where)
        # Keep a similar length, capped, so size-sensitive paths still see
        # long and short content without the fixture growing large.
        self.texts += 1
        unit = f"sanitized text {self.texts:04d} "
        length = max(len(unit), min(len(value), 1200))
        return (unit * (length // len(unit) + 1))[:length]

    def scrub(self, value, where, key="input"):
        """Free-form tool input: keep its shape, replace every string."""
        if isinstance(value, str):
            return f"sanitized-{key}"
        if isinstance(value, dict):
            return {k: self.scrub(v, f"{where}.{k}", k) for k, v in value.items()}
        if isinstance(value, list):
            return [self.scrub(v, f"{where}[]", key) for v in value]
        if value is None or isinstance(value, (bool, int, float)):
            return value
        raise Unreviewed(f"{where}: unexpected {type(value).__name__} value")

    def tool_id(self, value, where, _=None):
        return self.fake_id(expect(value, (str,), where), "toolu")

    def message_id(self, value, where, _=None):
        return self.fake_id(expect(value, (str,), where), "msg")

    def session_id(self, value, where, _=None):
        expect(value, (str,), where)
        return f"session-{self.fixture}"

    def placeholder(self, value, where, key):
        expect(value, (str,), where)
        return f"sanitized-{key}"

    def apply(self, schema, obj, where):
        if not isinstance(obj, dict):
            raise Unreviewed(f"{where}: expected an object, got {type(obj).__name__}")
        out = {}
        for key, value in obj.items():
            rule = schema.get(key)
            if rule is None:
                raise Unreviewed(f"{where}.{key}: unknown field; review it before sanitizing")
            out[key] = rule(value, f"{where}.{key}", key)
        return out

    def block(self, value, where, _=None):
        return self.apply({
            "text": self.text,
            "thinking": self.text,
            "signature": self.placeholder,
            "id": self.tool_id,
            "name": keep_str,
            "input": self.scrub,
            "tool_use_id": self.tool_id,
            "content": self.text,
            "is_error": keep_bool,
        }, value, where)

    def content(self, value, where, _=None):
        if value is None:
            return None
        expect(value, (list,), where)
        return [self.block(b, f"{where}[{i}]") for i, b in enumerate(value)]

    def message(self, value, where, _=None):
        return self.apply({
            "uuid": self.message_id,
            "role": keep_str,
            "content": self.content,
            "timestamp": keep_str,
            "api_error": self.text,
            "stop_reason": keep_str,
            "model": keep_str,
        }, value, where)

    def meta(self, value, where, _=None):
        return self.apply({
            "model": keep_str,
            "persona": keep_str,
            "turns": keep_int,
            "plan_mode": keep_bool,
            "tool_limit": keep_int,
        }, value, where)

    def record(self, value, where):
        return self.apply({
            "type": keep_str,
            "id": self.session_id,
            "created_at": keep_str,
            "model": keep_str,
            "persona": keep_str,
            "version": keep_int,
            "turns": keep_int,
            "plan_mode": keep_bool,
            "tool_limit": keep_int,
            "message": self.message,
            "meta": self.meta,
        }, value, where)


def main():
    if len(sys.argv) != 3:
        raise SystemExit(__doc__)
    path, fixture = sys.argv[1], sys.argv[2]
    sanitizer = Sanitizer(fixture)
    lines = []
    with open(path) as handle:
        for number, line in enumerate(handle, 1):
            if line.strip():
                try:
                    record = sanitizer.record(json.loads(line), f"line {number}")
                except Unreviewed as problem:
                    raise SystemExit(f"sanitize-session: {problem}")
                lines.append(json.dumps(record, separators=(",", ":")))
    # Print only after the whole file passed, so a refusal never leaves a
    # partial fixture behind on stdout.
    print("\n".join(lines))


if __name__ == "__main__":
    main()
