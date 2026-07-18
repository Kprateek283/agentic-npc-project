"""Test configuration.

Force LLM_PROVIDER=ollama before any project module imports config, so config loads
without a GEMINI_API_KEY (CI has none). No test here actually calls Ollama or any
network service — LLM and embedding calls are faked — so this only affects which client
objects config constructs at import, never a real request.
"""

import os

os.environ.setdefault("LLM_PROVIDER", "ollama")
