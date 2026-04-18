import base64
import os

from openai import OpenAI


def detect_image_mime(data: bytes) -> str | None:
    """Detect image MIME type from magic bytes."""
    if data[:8] == b"\x89PNG\r\n\x1a\n":
        return "image/png"
    if data[:3] == b"\xff\xd8\xff":
        return "image/jpeg"
    if data[:6] in (b"GIF87a", b"GIF89a"):
        return "image/gif"
    if data[:4] == b"RIFF" and data[8:12] == b"WEBP":
        return "image/webp"
    return None


def encode_image(path: str) -> tuple[str, str]:
    with open(path, "rb") as f:
        data = f.read()
    mime = detect_image_mime(data)
    if not mime:
        raise ValueError(f"unsupported image mime: {path}")
    return base64.b64encode(data).decode("utf-8"), mime


def extract_response_text(resp) -> str:
    if getattr(resp, "output_text", None):
        return resp.output_text
    dump = resp.model_dump() if hasattr(resp, "model_dump") else resp
    texts: list[str] = []
    for item in dump.get("output", []):
        if item.get("type") != "message":
            continue
        for part in item.get("content", []):
            if part.get("type") in ("output_text", "text") and part.get("text"):
                texts.append(part["text"])
    return "\n".join(texts).strip()


def main() -> None:
    api_key = os.getenv("TEST_ROUTER_API_KEY")
    if not api_key:
        raise ValueError("missing TEST_ROUTER_API_KEY")
    base_url = os.getenv("TEST_ROUTER_BASE_URL", "http://localhost:3011/v1")
    model = os.getenv("TEST_MODEL", "gpt-5.4")
    image_path = os.getenv("TEST_IMAGE_PATH", "/Users/liuxin2/Downloads/test.jpg")

    image_b64, image_mime = encode_image(image_path)
    client = OpenAI(api_key=api_key, base_url=base_url)

    # IMPORTANT: responses multimodal input should be wrapped as user message content.
    resp = client.responses.create(
        model=model,
        stream=False,
        input=[
            {
                "role": "user",
                "content": [
                    {"type": "input_text", "text": "what is in this image?"},
                    {
                        "type": "input_image",
                        "image_url": f"data:{image_mime};base64,{image_b64}",
                    },
                ],
            }
        ],
    )

    print("response_id:", resp.id)
    text = extract_response_text(resp)
    print("response_text:", text if text else "[empty]")


if __name__ == "__main__":
    main()
