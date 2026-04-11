# Router Python Test Examples

This directory provides two Python scripts for testing Router with the OpenAI Python SDK:

1. `test_nonstream.py`: non-stream (`stream=False`) call
2. `test_stream.py`: stream (`stream=True`) call

## Prerequisites

1. Python 3.10+
2. `openai` package installed:

```bash
pip install openai
```

3. Router service is running and reachable

## Required Environment Variables

Both scripts only read the following variables:

1. `TEST_ROUTER_BASE_URL`: Router API base URL (example: `http://localhost:3011/v1`)
2. `TEST_ROUTER_API_KEY`: API token for Router

If either variable is missing, the script exits with an error.

## Quick Start

Run non-stream test:

```bash
TEST_ROUTER_BASE_URL="http://localhost:3011/v1" \
TEST_ROUTER_API_KEY="<your_token>" \
python3 examples/test_nonstream.py
```

Run stream test:

```bash
TEST_ROUTER_BASE_URL="http://localhost:3011/v1" \
TEST_ROUTER_API_KEY="<your_token>" \
python3 examples/test_stream.py
```

## Optional Arguments

Both scripts accept optional positional arguments:

1. `model` (default: `gpt-5.4`)
2. `prompt` (default: `只输出OK`)

Example:

```bash
TEST_ROUTER_BASE_URL="http://localhost:3011/v1" \
TEST_ROUTER_API_KEY="<your_token>" \
python3 examples/test_nonstream.py "claude-opus-4-6" "只输出OK"
```

## Expected Output

Successful output includes:

1. model name
2. mode (`stream` or `non-stream`)
3. usage summary
4. final reply text

## Troubleshooting

1. `missing required env`:
   check `TEST_ROUTER_BASE_URL` and `TEST_ROUTER_API_KEY`.
2. `401 invalid token`:
   token is invalid or disabled.
3. `insufficient_user_quota`:
   account/group/channel quota is insufficient.
4. stream output empty:
   check channel model routing and upstream endpoint configuration.
