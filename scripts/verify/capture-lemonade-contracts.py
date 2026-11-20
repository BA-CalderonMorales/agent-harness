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

LEMONADE_MODEL defaults to the model the server has loaded with a window
smaller than its maximum (the one an oversized request can overflow), so
capturing does not force the server to swap models.
"""

import json
import os
import sys
import argparse
import urllib.parse
import http.client

BASE_URL = os.environ.get("LEMONADE_BASE_URL", "http://127.0.0.1:13305/v1").rstrip("/")
TIMEOUT = float(os.environ.get("LEMONADE_TIMEOUT", "300"))
FALLBACK_MODEL = "qwen3.5-4b-FLM"
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


def write_fixture(name, resp, data):
    lines = ["HTTP/1.1 %d %s" % (resp.status, resp.reason)]
    for key, value in resp.getheaders():
        if key.lower() in KEPT_HEADERS:
            lines.append("%s: %s" % (key, value))
    head = "\n".join(lines) + "\n\n"
    path = os.path.join(OUT_DIR, name)
    with open(path, "wb") as handle:
        handle.write(head.encode() + data)
    print("wrote %s (%d %s, %d body bytes)" % (path, resp.status, resp.reason, len(data)))


def pick_model(models):
    chosen = os.environ.get("LEMONADE_MODEL")
    if chosen:
        return chosen
    for entry in models:
        ctx = entry.get("context_length") or 0
        if ctx and ctx < (entry.get("max_context_window") or 0):
            return entry["id"]
    return FALLBACK_MODEL


def parse_args():
    parser = argparse.ArgumentParser(
        description="Capture Lemonade wire-format fixtures from a running local server.")
    parser.add_argument("--out", required=True, metavar="DIR",
                        help="directory to write the .http fixtures into")
    parser.add_argument("--overwrite-committed", action="store_true",
                        help="allow --out inside the committed testdata/contracts tree")
    return parser.parse_args()


def main():
    global OUT_DIR
    args = parse_args()
    OUT_DIR = os.path.realpath(args.out)
    inside = OUT_DIR == COMMITTED_DIR or OUT_DIR.startswith(COMMITTED_DIR + os.sep)
    if inside and not args.overwrite_committed:
        sys.exit("refusing to write into committed fixtures %s; pass --overwrite-committed" % COMMITTED_DIR)
    os.makedirs(OUT_DIR, exist_ok=True)

    resp, data = request("GET", "/models")
    if resp.status != 200:
        sys.exit("GET /models answered %d: is Lemonade running at %s?" % (resp.status, BASE_URL))
    write_fixture("models.http", resp, data)
    models = json.loads(data).get("data", [])
    model = pick_model(models)
    window = next((m.get("context_length") for m in models if m.get("id") == model), 4096) or 4096
    print("model under test: %s (context_length %d)" % (model, window))

    resp, data = request("POST", "/chat/completions", {
        "model": model,
        "stream": True,
        "max_tokens": 64,
        "messages": [{"role": "user", "content": "Reply with exactly: ready"}],
    })
    write_fixture("stream-text.http", resp, data)

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
    write_fixture("stream-tool-call.http", resp, data)

    # One word is at least one token, so window+512 words cannot fit.
    filler = " ".join(["alpha", "beta", "gamma", "delta"] * ((window + 512) // 4 + 1))
    resp, data = request("POST", "/chat/completions", {
        "model": model,
        "stream": True,
        "max_tokens": 16,
        "messages": [{"role": "user", "content": filler}],
    })
    write_fixture("overflow.http", resp, data)
    print("fixtures written to %s" % OUT_DIR)


if __name__ == "__main__":
    main()
