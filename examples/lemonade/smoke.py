#!/usr/bin/env python3
"""Check a local model's text, tool round trip, and image input; stdlib only."""

import argparse
import base64
import json
import re
import struct
import sys
import time
import urllib.error
import urllib.request
import zlib

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--endpoint", default="http://127.0.0.1:13305/v1")
parser.add_argument("--model", default="qwen3.5-9b-FLM")
parser.add_argument("--timeout", type=float, default=180)
parser.add_argument("--skip-vision", action="store_true")
args = parser.parse_args()

# A solid red PNG generated in memory. No files or real screenshots are sent.
pixels = b"".join(b"\x00" + b"\xff\x00\x00" * 64 for _ in range(64))
png = bytearray(b"\x89PNG\r\n\x1a\n")
for kind, data in [
    (b"IHDR", struct.pack(">IIBBBBB", 64, 64, 8, 2, 0, 0, 0)),
    (b"IDAT", zlib.compress(pixels)),
    (b"IEND", b""),
]:
    png.extend(struct.pack(">I", len(data)) + kind + data)
    png.extend(struct.pack(">I", zlib.crc32(kind + data) & 0xFFFFFFFF))
image_url = "data:image/png;base64," + base64.b64encode(png).decode()

tool = {
    "type": "function",
    "function": {
        "name": "get_check_value",
        "description": "Return the check value requested by the user.",
        "parameters": {
            "type": "object", "properties": {}, "additionalProperties": False,
        },
    },
}
history = [{"role": "user", "content":
            "Call get_check_value, then reply with exactly its value."}]
text = [{"role": "user", "content": "Reply with exactly: Local inference works"}]
vision = [{"role": "user", "content": [
    {"type": "text", "text": "What is the solid image's color? Reply with one color word."},
    {"type": "image_url", "image_url": {"url": image_url}},
]}]

steps = ["text", "tool call", "tool result"]
if not args.skip_vision:
    steps.append("vision")

failures = []
tool_call_valid = False
for step in steps:
    if step == "tool result" and not tool_call_valid:
        print("SKIP tool result: tool-call prerequisite failed", flush=True)
        continue
    payload = {
        "model": args.model, "max_tokens": 1024, "stream": False,
        "messages": text if step == "text" else vision if step == "vision" else history,
    }
    if step.startswith("tool"):
        payload["tools"] = [tool]
        payload["tool_choice"] = "required" if step == "tool call" else "none"
    request = urllib.request.Request(
        args.endpoint.rstrip("/") + "/chat/completions",
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
    )
    started = time.monotonic()
    try:
        with urllib.request.urlopen(request, timeout=args.timeout) as response:
            body = json.load(response)
        choice = body["choices"][0]
        message = choice["message"]
        content = message.get("content") or ""
        if choice.get("finish_reason") == "length":
            raise ValueError("response exhausted its output budget")
        if step == "text" and content.strip() != "Local inference works":
            raise ValueError("unexpected text response: " + repr(content))
        if step == "tool call":
            calls = message.get("tool_calls", [])
            if len(calls) != 1:
                raise ValueError("expected exactly one tool call")
            call = calls[0]
            if call["function"]["name"] != "get_check_value" or not call.get("id"):
                raise ValueError("unexpected tool name or missing call ID")
            if json.loads(call["function"]["arguments"]) != {}:
                raise ValueError("unexpected tool arguments")
            history.extend([message, {
                "role": "tool", "tool_call_id": call["id"],
                "content": json.dumps({"value": "HARNESS_OK_731"}),
            }])
            tool_call_valid = True
        if step == "tool result" and content.strip() != "HARNESS_OK_731":
            if "HARNESS_OK_731" in content:
                raise ValueError("exact-format mismatch (expected value is present): " + repr(content))
            raise ValueError("expected tool result value is missing: " + repr(content))
        if step == "vision" and not re.search(r"\bred\b", content.lower()):
            raise ValueError("unexpected color response: " + repr(content))
        print(f"PASS {step}: {time.monotonic() - started:.2f}s", flush=True)
    except urllib.error.HTTPError as error:
        detail = error.read(4096).decode(errors="replace")
        failures.append(step)
        print(f"FAIL {step}: HTTP {error.code}: {detail}", file=sys.stderr, flush=True)
    except (OSError, ValueError, KeyError, IndexError, TypeError) as error:
        failures.append(step)
        print(f"FAIL {step}: {time.monotonic() - started:.2f}s: {error}", file=sys.stderr, flush=True)

if failures:
    print("Smoke checks failed: " + ", ".join(failures) +
          ". Independent checks continued; the exit status remains nonzero.", file=sys.stderr)
    sys.exit(1)
print("Smoke checks passed. Coding quality, streaming, and recovery remain untested.")
