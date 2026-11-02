"""Check the published curl examples offline, without compiling or sending requests."""

import html
import json
import os
from pathlib import Path
import re
import subprocess


def main():
    source = (Path(__file__).resolve().parents[1] / "frontend/public/docs/quickstart.html").read_text()
    examples = [html.unescape(code) for code in re.findall(r"<pre><code>(.*?)</code></pre>", source, re.S)]
    assert len(examples) == 2, "Expected Messages and Chat Completions examples"
    # A shell function captures arguments; curl never reaches the network.
    capture = "curl() { printf '%s\\0' \"$@\"; }\n"
    env = os.environ.copy()
    dummy_key = "offline-placeholder with spaces $literal"
    env["TASKSETLY_API_KEY"] = dummy_key
    for example, endpoint, header in zip(examples, ("messages", "chat/completions"), (
        f"x-api-key: {dummy_key}", f"authorization: Bearer {dummy_key}",
    )):
        subprocess.run(["bash", "-n"], input=example, text=True, check=True)
        result = subprocess.run(["bash", "--noprofile", "--norc", "-c", capture + example],
                                env=env, capture_output=True, check=True)
        args = result.stdout.decode().split("\0")[:-1]
        assert args[0] == f"https://ai.tasksetly.com/v1/{endpoint}"
        headers = [args[i + 1] for i, arg in enumerate(args) if arg == "-H"]
        assert header in headers, "Credential expansion must remain one header argument"
        assert "content-type: application/json" in headers
        if endpoint == "messages":
            assert "anthropic-version: 2023-06-01" in headers
        body = json.loads(args[args.index("-d") + 1])
        assert body["model"] == "MODEL_ID_FROM_CONSOLE"
        assert body["messages"] == [{"role": "user", "content": "Reply with OK"}]
        for missing_key in (None, ""):
            missing_env = env.copy()
            missing_env.pop("TASKSETLY_API_KEY")
            if missing_key is not None:
                missing_env["TASKSETLY_API_KEY"] = missing_key
            blocked = subprocess.run(["bash", "--noprofile", "--norc", "-c", capture + example],
                                     env=missing_env, capture_output=True)
            assert blocked.returncode != 0 and not blocked.stdout, "Missing key must stop before curl"
    print("Quickstart checks passed: shell syntax, headers, JSON and missing-key guards (offline).")


if __name__ == "__main__":
    main()
