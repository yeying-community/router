#!/usr/bin/env python3
import os
import sys
from openai import OpenAI


def require_env(name: str) -> str:
    value = os.getenv(name, "").strip()
    if not value:
        raise RuntimeError(f"missing required env: {name}")
    return value


def build_client() -> tuple[OpenAI, str]:
    base_url = require_env("TEST_ROUTER_BASE_URL")
    api_key = require_env("TEST_ROUTER_API_KEY")
    return OpenAI(api_key=api_key, base_url=base_url), api_key


def normalize_content(content) -> str:
    if content is None:
        return ""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        for item in content:
            if isinstance(item, dict) and item.get("type") in ("text", "output_text"):
                text = item.get("text")
                if isinstance(text, str) and text:
                    parts.append(text)
        return "".join(parts)
    return str(content)


def run_nonstream(model: str, prompt: str) -> str:
    client, api_key = build_client()
    resp = client.chat.completions.create(
        model=model,
        messages=[
            {"role": "system", "content": "You are a helpful assistant."},
            {"role": "user", "content": prompt},
        ],
        stream=False,
    )
    text = ""
    if getattr(resp, "choices", None):
        message = resp.choices[0].message
        text = normalize_content(getattr(message, "content", ""))

    print(f"model={model}")
    print("mode=non-stream")
    print(f"usage={getattr(resp, 'usage', None)}")
    print(f"key_prefix={api_key[:8]}")
    return text.strip()


if __name__ == "__main__":
    model = sys.argv[1] if len(sys.argv) > 1 else "gpt-5.4"
    prompt = sys.argv[2] if len(sys.argv) > 2 else "只输出OK"
    try:
        result = run_nonstream(model, prompt)
        print("reply:")
        print(result if result else "[empty]")
    except Exception as exc:
        print(f"error: {exc}")
        raise
