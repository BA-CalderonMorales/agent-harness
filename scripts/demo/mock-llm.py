#!/usr/bin/env python3
"""Mock OpenAI-compatible LLM server for TUI demo recordings.

Serves /v1/models (readiness) and /v1/chat/completions (streamed
demo responses) so agent-harness boots green-backed without a real
model. Run it before recording:

    python3 scripts/demo/mock-llm.py
"""
import json
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PORT = 8080
MODEL = "demo-1.0"
# The local provider's pinned default (config defaults); the demo model
# list includes it so the login wizard's picker shows the [default] pin.
DEFAULT_MODEL = "deepreinforce-ai/Ornith-1.0-9B-GGUF"
CHUNK_DELAY = 0.25
WELCOME = "Hello! Agent-harness is live: this agent loop can read your workspace, run shell commands, and stream every result straight into the conversation."


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def _json(self, payload, status=200):
        body = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path.rstrip("/").endswith("/models"):
            self._json({"object": "list", "data": [
                {"id": DEFAULT_MODEL, "object": "model", "owned_by": "agent-harness-demo"},
                {"id": MODEL, "object": "model", "owned_by": "agent-harness-demo"},
            ]})
            return
        self._json({"error": "not found"}, 404)

    def _has_tool_messages(self, body):
        return any(m.get("role") == "tool" for m in body.get("messages", []))

    def do_POST(self):
        if self.path.rstrip("/").endswith("/chat/completions"):
            length = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(length) or b"{}")
            for message in reversed(body.get("messages", [])):
                if message.get("role") == "user":
                    print("PYLOAD: " + str(message.get("content", ""))[:120], flush=True)
                    break
            for message in body.get("messages", []):
                if message.get("role") == "tool":
                    print("PYTOOL: " + str(message.get("content", ""))[:200], flush=True)
                    break
            # Tool-burst demo: a user message starting with "burst20"
            # triggers 20 real bash tool calls (echo), exercising the
            # tool-run collapse. The follow-up request (tool results)
            # gets the normal welcome stream.
            last_user = ""
            for message in reversed(body.get("messages", [])):
                if message.get("role") == "user":
                    content = message.get("content", "")
                    if isinstance(content, list):
                        content = " ".join(
                            b.get("text", "") for b in content if isinstance(b, dict)
                        )
                    last_user = str(content)
                    break
            # 15 calls: the agent loop's MaxToolCalls default (15) is the
            # real ceiling; a larger burst trips the convergence guard.
            if last_user.startswith("burst20") and not self._has_tool_messages(body):
                calls = [
                    {
                        "index": i,
                        "id": f"tb{i}",
                        "type": "function",
                        "function": {
                            "name": "bash",
                            "arguments": f'{{"command": "echo burst-{i}"}}',
                        },
                    }
                    for i in range(15)
                ]
                chunk = {"choices": [{"delta": {"tool_calls": calls}, "finish_reason": "tool_calls"}]}
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.end_headers()
                self.wfile.write(("data: " + json.dumps(chunk) + "\n\n").encode())
                self.wfile.flush()
                self.wfile.write(b"data: [DONE]\n\n")
                self.wfile.flush()
                return
            # "think" streams reasoning_content deltas before the
            # answer: GLM/DeepSeek-style thinking models, so the
            # reasoning preview and its expanded frame are reproducible
            # in the live rig.
            if last_user.startswith("think"):
                try:
                    for line in (
                        "The user wants the repository layout.",
                        "I should describe the top-level directories,",
                        "then how the agent loop reaches the TUI.",
                    ):
                        chunk = {"choices": [{"delta": {"reasoning_content": line + "\n"}}]}
                        self.wfile.write(("data: " + json.dumps(chunk) + "\n\n").encode())
                        self.wfile.flush()
                        time.sleep(CHUNK_DELAY)
                except (BrokenPipeError, ConnectionResetError):
                    pass
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            try:
                for piece in WELCOME.split(" "):
                    chunk = {"choices": [{"delta": {"content": piece + " "}}]}
                    self.wfile.write(("data: " + json.dumps(chunk) + "\n\n").encode())
                    self.wfile.flush()
                    time.sleep(CHUNK_DELAY)
                # A final chunk with finish_reason="stop" is mandatory: the
                # SSE reader treats a stream that ends without it as an empty
                # message and the whole turn is lost (stuck thinking header).
                finish = {"choices": [{"delta": {}, "finish_reason": "stop"}]}
                self.wfile.write(("data: " + json.dumps(finish) + "\n\n").encode())
                self.wfile.flush()
                self.wfile.write(b"data: [DONE]\n\n")
                self.wfile.flush()
            except (BrokenPipeError, ConnectionResetError):
                # The client hung up mid-stream (probe timeout, tab
                # switch): the demo server must survive and serve the
                # next request.
                pass
            return
        self._json({"error": "not found"}, 404)


if __name__ == "__main__":
    ThreadingHTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
