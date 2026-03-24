"""
qdrant_loader.py — Load embedded chunks into a Qdrant collection.

Handles:
  - Collection creation (idempotent)
  - Batch upsert with payload
  - Optional wipe-and-reload (--reset flag in ingest.py)
"""

import uuid
from typing import List, Tuple

from qdrant_client import QdrantClient
from qdrant_client.models import (
    Distance,
    VectorParams,
    PointStruct,
)

from chunker import Chunk


BATCH_SIZE = 64  # upsert this many points per request


class QdrantLoader:
    def __init__(
        self,
        host: str = "localhost",
        port: int = 6333,
        collection: str = "constitution",
        vector_size: int = 768,
        distance: Distance = Distance.COSINE,
    ):
        self.client = QdrantClient(host=host, port=port)
        self.collection = collection
        self.vector_size = vector_size
        self.distance = distance

    # ------------------------------------------------------------------
    # Collection management
    # ------------------------------------------------------------------

    def ensure_collection(self, reset: bool = False):
        """Create the collection if it doesn't exist. If reset=True, drop and recreate."""
        exists = any(
            c.name == self.collection
            for c in self.client.get_collections().collections
        )

        if exists and reset:
            print(f"[loader] Dropping existing collection '{self.collection}' …")
            self.client.delete_collection(self.collection)
            exists = False

        if not exists:
            print(f"[loader] Creating collection '{self.collection}' (size={self.vector_size}, distance={self.distance}) …")
            self.client.create_collection(
                collection_name=self.collection,
                vectors_config=VectorParams(
                    size=self.vector_size,
                    distance=self.distance,
                ),
            )
        else:
            print(f"[loader] Collection '{self.collection}' already exists — appending.")

    # ------------------------------------------------------------------
    # Upsert
    # ------------------------------------------------------------------

    def load(self, chunks: List[Chunk], embeddings: List[List[float]]):
        """Upsert all (chunk, embedding) pairs into Qdrant in batches."""
        assert len(chunks) == len(embeddings), "chunks and embeddings must have the same length"

        total = len(chunks)
        uploaded = 0

        for start in range(0, total, BATCH_SIZE):
            batch_chunks = chunks[start : start + BATCH_SIZE]
            batch_vecs = embeddings[start : start + BATCH_SIZE]

            points = [
                PointStruct(
                    id=str(uuid.uuid4()),
                    vector=vec,
                    payload={**chunk.metadata, "text": chunk.text},
                )
                for chunk, vec in zip(batch_chunks, batch_vecs)
            ]

            self.client.upsert(collection_name=self.collection, points=points)
            uploaded += len(points)
            print(f"[loader] Uploaded {uploaded}/{total} points …", end="\r")

        print(f"\n[loader] Done. {uploaded} points in '{self.collection}'.")

    # ------------------------------------------------------------------
    # Info helpers
    # ------------------------------------------------------------------

    def collection_info(self) -> dict:
        info = self.client.get_collection(self.collection)
        return {
            "name": self.collection,
            "indexed_vectors_count": info.indexed_vectors_count,
            "points_count": info.points_count,
            "status": str(info.status),
        }
