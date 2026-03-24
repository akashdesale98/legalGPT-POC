"""
embed.py — Generate embeddings via Ollama or Cohere.

Preserved: core Ollama HTTP call logic from POC embedder.py.
Added: config-driven provider selection, retry with exponential backoff.
"""

import time
import requests
from typing import List, Optional


class OllamaEmbedder:
    """Embed via Ollama /api/embeddings endpoint. Preserved from POC."""

    def __init__(self, base_url: str = "http://localhost:11434", model: str = "nomic-embed-text"):
        self.base_url = base_url.rstrip("/")
        self.model = model
        self._verify_model()

    def _verify_model(self):
        try:
            resp = requests.get(f"{self.base_url}/api/tags", timeout=5)
            resp.raise_for_status()
            models = [m["name"].split(":")[0] for m in resp.json().get("models", [])]
            model_base = self.model.split(":")[0]
            if model_base not in models:
                print(f"[embedder] WARNING: model '{self.model}' not found. Available: {models}")
        except requests.RequestException as e:
            print(f"[embedder] WARNING: cannot reach Ollama at {self.base_url} — {e}")

    def embed(self, text: str) -> List[float]:
        resp = requests.post(
            f"{self.base_url}/api/embeddings",
            json={"model": self.model, "prompt": text},
            timeout=120,
        )
        resp.raise_for_status()
        return resp.json()["embedding"]

    def embed_batch(self, texts: List[str], batch_size: int = 32,
                    show_progress: bool = True, max_retries: int = 3) -> List[List[float]]:
        embeddings = []
        total = len(texts)
        for i, text in enumerate(texts):
            if show_progress and (i % 10 == 0 or i == total - 1):
                print(f"[embedder] {i + 1}/{total} embedded …", end="\r")

            embedding = None
            for attempt in range(max_retries):
                try:
                    embedding = self.embed(text)
                    break
                except (requests.RequestException, KeyError) as e:
                    if attempt < max_retries - 1:
                        wait = 2 ** attempt
                        print(f"\n[embedder] Retry {attempt + 1}/{max_retries} after {wait}s: {e}")
                        time.sleep(wait)
                    else:
                        raise RuntimeError(f"Failed to embed at index {i}: {e}") from e

            if embedding is None:
                raise RuntimeError(f"No embedding for index {i}")

            if len(embeddings) == 0:
                print(f"\n[embedder] Dimensions: {len(embedding)}")
            embeddings.append(embedding)

        if show_progress:
            print()
        return embeddings


class CohereEmbedder:
    """Embed via Cohere Embed API."""

    def __init__(self, api_key: str, model: str = "embed-multilingual-v3.0"):
        self.api_key = api_key
        self.model = model

    def embed_batch(self, texts: List[str], batch_size: int = 96,
                    input_type: str = "search_document", show_progress: bool = True,
                    max_retries: int = 3) -> List[List[float]]:
        all_embeddings = []
        total = len(texts)
        for start in range(0, total, batch_size):
            batch = texts[start: start + batch_size]
            if show_progress:
                print(f"[embedder] Cohere {start + 1}-{min(start + batch_size, total)}/{total}")

            for attempt in range(max_retries):
                try:
                    resp = requests.post(
                        "https://api.cohere.ai/v1/embed",
                        json={"texts": batch, "model": self.model, "input_type": input_type},
                        headers={"Authorization": f"Bearer {self.api_key}"},
                        timeout=60,
                    )
                    if resp.status_code == 429:
                        time.sleep(2 ** attempt)
                        continue
                    resp.raise_for_status()
                    all_embeddings.extend(resp.json()["embeddings"])
                    break
                except requests.RequestException as e:
                    if attempt < max_retries - 1:
                        time.sleep(2 ** attempt)
                    else:
                        raise RuntimeError(f"Cohere embed failed: {e}") from e
        return all_embeddings


def create_embedder(provider: str = "ollama", ollama_url: str = "http://localhost:11434",
                    model: str = "nomic-embed-text", cohere_api_key: Optional[str] = None):
    if provider == "cohere":
        if not cohere_api_key:
            raise ValueError("COHERE_API_KEY required for Cohere embedder")
        return CohereEmbedder(api_key=cohere_api_key, model=model)
    return OllamaEmbedder(base_url=ollama_url, model=model)


if __name__ == "__main__":
    embedder = OllamaEmbedder()
    vec = embedder.embed("What are the fundamental rights in the Indian Constitution?")
    print(f"Vector dimensions: {len(vec)}")
