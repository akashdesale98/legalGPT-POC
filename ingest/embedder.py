"""
embedder.py — Generate embeddings via Ollama's local API.

Supports any Ollama embedding model (default: nomic-embed-text → 768 dims).
Returns float32 vectors ready for Qdrant ingestion.
"""

import requests
from typing import List


class OllamaEmbedder:
    """Thin wrapper around Ollama /api/embeddings endpoint."""

    def __init__(self, base_url: str = "http://localhost:11434", model: str = "nomic-embed-text"):
        self.base_url = base_url.rstrip("/")
        self.model = model
        self._verify_model()

    def _verify_model(self):
        """Check Ollama is running and the model is available."""
        try:
            resp = requests.get(f"{self.base_url}/api/tags", timeout=5)
            resp.raise_for_status()
            models = [m["name"].split(":")[0] for m in resp.json().get("models", [])]
            model_base = self.model.split(":")[0]
            if model_base not in models:
                print(
                    f"[embedder] WARNING: model '{self.model}' not found in Ollama. "
                    f"Available: {models}. "
                    f"Run: ollama pull {self.model}"
                )
        except requests.RequestException as e:
            print(f"[embedder] WARNING: cannot reach Ollama at {self.base_url} — {e}")

    def embed(self, text: str) -> List[float]:
        """Return embedding vector for a single text string."""
        resp = requests.post(
            f"{self.base_url}/api/embeddings",
            json={"model": self.model, "prompt": text},
            timeout=120,
        )
        resp.raise_for_status()
        return resp.json()["embedding"]

    def embed_batch(self, texts: List[str], show_progress: bool = True) -> List[List[float]]:
        """
        Embed a list of texts, returning a list of vectors.
        Ollama doesn't support true batching, so we loop with progress reporting.
        """
        embeddings = []
        total = len(texts)
        for i, text in enumerate(texts):
            if show_progress and (i % 10 == 0 or i == total - 1):
                print(f"[embedder] {i + 1}/{total} embedded …", end="\r")
            embeddings.append(self.embed(text))
        if show_progress:
            print()  # newline after progress
        return embeddings


if __name__ == "__main__":
    embedder = OllamaEmbedder()
    vec = embedder.embed("What are the fundamental rights in the Indian Constitution?")
    print(f"Vector dimensions: {len(vec)}")
    print(f"First 5 values: {vec[:5]}")
