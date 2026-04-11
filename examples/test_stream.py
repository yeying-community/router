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


def run_stream(model: str, prompt: str) -> str:
    client, api_key = build_client()
    stream = client.chat.completions.create(
        model=model,
        messages=[
            {"role": "system", "content": "You are a helpful assistant."},
            {"role": "user", "content": prompt},
        ],
        stream=True,
    )
    text_chunks = []
    usage = None
    for chunk in stream:
        if getattr(chunk, "choices", None):
            delta = chunk.choices[0].delta
            content = getattr(delta, "content", None)
            if content:
                text_chunks.append(content)
        if getattr(chunk, "usage", None):
            usage = chunk.usage

    text = "".join(text_chunks).strip()
    print(f"model={model}")
    print("mode=stream")
    print(f"usage={usage}")
    print(f"key_prefix={api_key[:8]}")
    return text


if __name__ == "__main__":
    model = sys.argv[1] if len(sys.argv) > 1 else "gpt-5.4"
    prompt = sys.argv[2] if len(sys.argv) > 2 else "只输出OK"
    try:
        result = run_stream(model, prompt)
        print("reply:")
        print(result if result else "[empty]")
    except Exception as exc:
        print(f"error: {exc}")
        raise
