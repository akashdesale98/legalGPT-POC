"""
config.py — Pydantic BaseSettings for ingest pipeline configuration.

All values read from environment variables / .env file.
No hardcoded defaults for secrets.
"""

import os
from typing import Optional


class IngestConfig:
    """Configuration for the ingest pipeline. Reads from environment variables."""

    def __init__(self):
        self.qdrant_url: str = os.getenv("QDRANT_URL", "http://localhost:6333")
        self.qdrant_api_key: Optional[str] = os.getenv("QDRANT_API_KEY")
        self.qdrant_collection: str = os.getenv("QDRANT_COLLECTION", "constitution")
        self.embedding_provider: str = os.getenv("EMBEDDING_PROVIDER", "ollama")  # ollama | cohere
        self.ollama_url: str = os.getenv("OLLAMA_URL", "http://localhost:11434")
        self.cohere_api_key: Optional[str] = os.getenv("COHERE_API_KEY")
        self.embed_model: str = os.getenv("EMBED_MODEL", "nomic-embed-text")
        self.chunk_size: int = int(os.getenv("CHUNK_SIZE", "400"))
        self.chunk_overlap: int = int(os.getenv("CHUNK_OVERLAP", "64"))
        self.batch_size: int = int(os.getenv("BATCH_SIZE", "32"))
        self.log_level: str = os.getenv("LOG_LEVEL", "INFO")
        self.failed_docs_dir: str = os.getenv("FAILED_DOCS_DIR", "failed_docs")

    def validate(self) -> None:
        """Validate that required configuration is present."""
        if self.embedding_provider == "cohere" and not self.cohere_api_key:
            raise ValueError("COHERE_API_KEY is required when EMBEDDING_PROVIDER=cohere")
