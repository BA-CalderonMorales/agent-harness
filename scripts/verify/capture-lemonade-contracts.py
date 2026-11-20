#!/usr/bin/env python3
"""Capture Lemonade wire-format fixtures for the provider contract tests.

Opt-in and never run in CI: it talks to a real local Lemonade server
(default http://127.0.0.1:13305/v1) and writes fixtures into the
directory given by the required --out. Writing into the committed
internal/runtime/llm/testdata/contracts/ tree needs --overwrite-committed.

Each fixture is a raw HTTP response (status line, a few stable headers,
blank line, body) that Go's http.ReadResponse can parse, so the contract
tests replay exactly what the server sent through the real HTTPClient.

Every prompt is synthetic. Usage:

    python3 scripts/verify/capture-lemonade-contracts.py --out "$(mktemp -d)"
    LEMONADE_BASE_URL=http://127.0.0.1:13305/v1 \
    LEMONADE_MODEL=qwen3.5-4b-FLM python3 scripts/verify/capture-lemonade-contracts.py --out DIR

The model is --model (or LEMONADE_MODEL) and must be listed in /models.
Without one, the script picks the model whose loaded context_length is
below its max_context_window (the one an oversized request can overflow);
if none qualifies it exits and asks for --model. It never guesses a name.

Every response is validated before anything is written: a failed capture
exits non-zero and leaves the output directory untouched.
"""

import json
import os
import sys
import argparse
import urllib.parse
import http.client

BASE_URL = os.environ.get("LEMONADE_BASE_URL", "http://127.0.0.1:13305/v1").rstrip("/")
TIMEOUT = float(os.environ.get("LEMONADE_TIMEOUT", "300"))
COMMITTED_DIR = os.path.realpath(os.path.join(
    os.path.dirname(os.path.abspath(__file__)),
    "..", "..", "internal", "runtime", "llm", "testdata", "contracts",
))
OUT_DIR = None  # set from --out in main()

# Headers that the client reads or that describe the body. Everything
# else (dates, keep-alive, CORS) is volatile noise.
KEPT_HEADERS = {"content-type", "retry-after"}

WEATHER_TOOL = {
    "type": "function",
    "function": {
        "name": "get_weather",
        "description": "Get the weather for a city",
        "parameters": {
            "type": "object",
            "properties": {"city": {"type": "string"}},
            "required": ["city"],
        },
    },
}


def request(method, path, body=None):
    url = urllib.parse.urlparse(BASE_URL + path)
    conn = http.client.HTTPConnection(url.hostname, url.port or 80, timeout=TIMEOUT)
    headers = {"Content-Type": "application/json"} if body is not None else {}
    payload = json.dumps(body).encode() if body is not None else None
    conn.request(method, url.path, body=payload, headers=headers)
    resp = conn.getresponse()
    data = resp.read()
    conn.close()
    return resp, data


class CaptureError(Exception):
    pass


def render_fixture(resp, data):
    lines = ["HTTP/1.1 %d %s" % (resp.status, resp.reason)]
    for key, value in resp.getheaders():
        if key.lower() in KEPT_HEADERS:
            lines.append("%s: %s" % (key, value))
    return ("\n".join(lines) + "\n\n").encode() + data


def check_models(resp, data):
    if resp.status != 200:
        raise CaptureError("GET /models answered %d" % resp.status)
    try:
        models = json.loads(data).get("data")
    except (ValueError, AttributeError):
        raise CaptureError("GET /models did not return a JSON object")
    if not isinstance(models, list) or not models:
        raise CaptureError("GET /models listed no models")
    return models


def check_stream(what, resp, data):
    if resp.status != 200:
        raise CaptureError("%s answered %d: %s" % (what, resp.status, data[:200]))
    if b"data:" not in data:
        raise CaptureError("%s returned no SSE data lines" % what)
    if b'"error"' in data:
        raise CaptureError("%s returned an error instead of a stream: %s" % (what, data[:200]))


def check_overflow(resp, data):
    if resp.status != 200 or b'"error"' not in data:
        raise CaptureError(
            "the oversized request did not produce the in-band overflow error "
            "(status %d): %s" % (resp.status, data[:200]))


def pick_model(models, explicit):
    ids = [m.get("id") for m in models]
    if explicit:
        if explicit not in ids:
            raise CaptureError("model %r is not listed in /models (listed: %s)" % (explicit, ", ".join(map(str, ids))))
        return explicit
    for entry in models:
        ctx = entry.get("context_length") or 0
        if ctx and ctx < (entry.get("max_context_window") or 0):
            return entry["id"]
    raise CaptureError("no listed model reports a reduced context window; pass --model one of: " + ", ".join(map(str, ids)))


def parse_args():
    parser = argparse.ArgumentParser(
        description="Capture Lemonade wire-format fixtures from a running local server.")
    parser.add_argument("--out", required=True, metavar="DIR",
                        help="directory to write the .http fixtures into")
    parser.add_argument("--model", default=os.environ.get("LEMONADE_MODEL"),
                        help="model to capture with; must be listed in /models (default: LEMONADE_MODEL)")
    parser.add_argument("--overwrite-committed", action="store_true",
                        help="allow --out inside the committed testdata/contracts tree")
    return parser.parse_args()


def capture(model_arg):
    """Return {filename: bytes} for every fixture, or raise CaptureError."""
    fixtures = {}
    resp, data = request("GET", "/models")
    models = check_models(resp, data)
    fixtures["models.http"] = render_fixture(resp, data)

    model = pick_model(models, model_arg)
    window = next((m.get("context_length") for m in models if m.get("id") == model), 0) or 4096
    print("model under test: %s (context_length %d)" % (model, window))

    resp, data = request("POST", "/chat/completions", {
        "model": model,
        "stream": True,
        "max_tokens": 64,
        "messages": [{"role": "user", "content": "Reply with exactly: ready"}],
    })
    check_stream("the text chat", resp, data)
    fixtures["stream-text.http"] = render_fixture(resp, data)

    resp, data = request("POST", "/chat/completions", {
        "model": model,
        "stream": True,
        "max_tokens": 256,
        "tools": [WEATHER_TOOL],
        "messages": [{
            "role": "user",
            "content": "Use the get_weather tool for the city Paris. Do not answer in text.",
        }],
    })
    check_stream("the tool-call chat", resp, data)
    fixtures["stream-tool-call.http"] = render_fixture(resp, data)

    # One word is at least one token, so window+512 words cannot fit.
    filler = " ".join(["alpha", "beta", "gamma", "delta"] * ((window + 512) // 4 + 1))
    resp, data = request("POST", "/chat/completions", {
        "model": model,
        "stream": True,
        "max_tokens": 16,
        "messages": [{"role": "user", "content": filler}],
    })
    check_overflow(resp, data)
    fixtures["overflow.http"] = render_fixture(resp, data)
    return fixtures


def main():
    global OUT_DIR
    args = parse_args()
    OUT_DIR = os.path.realpath(args.out)
    inside = OUT_DIR == COMMITTED_DIR or OUT_DIR.startswith(COMMITTED_DIR + os.sep)
    if inside and not args.overwrite_committed:
        sys.exit("refusing to write into committed fixtures %s; pass --overwrite-committed" % COMMITTED_DIR)

    try:
        fixtures = capture(args.model)
    except CaptureError as err:
        sys.exit("capture failed, nothing written: %s" % err)
    except OSError as err:
        sys.exit("capture failed, nothing written: cannot reach Lemonade at %s: %s" % (BASE_URL, err))

    os.makedirs(OUT_DIR, exist_ok=True)
    for name, content in fixtures.items():
        path = os.path.join(OUT_DIR, name)
        with open(path, "wb") as handle:
            handle.write(content)
        print("wrote %s (%d bytes)" % (path, len(content)))
    print("fixtures written to %s" % OUT_DIR)


if __name__ == "__main__":
    main()
