"""
qdrant_client.py — Load embedded chunks into Qdrant with retry and idempotent upsert.

Preserved: core upsert logic from POC qdrant_loader.py.
Added: retry on connection error, batch size 100, collection creation if not exists.
"""

import time
import uuid
from typing import List, Optional, Tuple

from qdrant_client import QdrantClient
from qdrant_client.models import (
    Distance,
    VectorParams,
    SparseVectorParams,
    SparseIndexParams,
    PointStruct,
    SparseVector,
)

from ingest.chunker.statute import Chunk


BATCH_SIZE = 100
MAX_RETRIES = 3
SPARSE_VECTOR_NAME = "bm25"


class QdrantLoader:
    """Manages Qdrant collection creation and batch upsert. Preserved from POC."""

    def __init__(
        self,
        host: str = "localhost",
        port: int = 6333,
        collection: str = "constitution",
        vector_size: int = 768,
        distance: Distance = Distance.COSINE,
        api_key: Optional[str] = None,
    ):
        kwargs = {"host": host, "port": port}
        if api_key:
            kwargs["api_key"] = api_key
        self.client = QdrantClient(**kwargs)
        self.collection = collection
        self.vector_size = vector_size
        self.distance = distance

    def ensure_collection(self, reset: bool = False, with_sparse: bool = True):
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
            print(f"[loader] Creating collection '{self.collection}' (size={self.vector_size}, sparse={with_sparse}) …")
            sparse_vectors_config = None
            if with_sparse:
                sparse_vectors_config = {
                    SPARSE_VECTOR_NAME: SparseVectorParams(
                        index=SparseIndexParams(on_disk=False)
                    )
                }
            self.client.create_collection(
                collection_name=self.collection,
                vectors_config={"dense": VectorParams(
                    size=self.vector_size,
                    distance=self.distance,
                )},
                sparse_vectors_config=sparse_vectors_config,
            )
        else:
            print(f"[loader] Collection '{self.collection}' already exists — appending.")

    def load(
        self,
        chunks: List[Chunk],
        embeddings: List[List[float]],
        sparse_vectors: Optional[List[Tuple[List[int], List[float]]]] = None,
    ):
        """Upsert all (chunk, embedding) pairs with retry logic.

        sparse_vectors: optional list of (indices, values) per chunk.
        If provided, stored as the 'bm25' sparse vector field.
        """
        if len(chunks) != len(embeddings):
            raise ValueError(f"chunks ({len(chunks)}) and embeddings ({len(embeddings)}) must have the same length")

        total = len(chunks)
        uploaded = 0

        for start in range(0, total, BATCH_SIZE):
            batch_chunks = chunks[start: start + BATCH_SIZE]
            batch_vecs = embeddings[start: start + BATCH_SIZE]

            batch_sparse = (
                sparse_vectors[start: start + BATCH_SIZE]
                if sparse_vectors else None
            )

            points = []
            for i, (chunk, vec) in enumerate(zip(batch_chunks, batch_vecs)):
                # Use content hash as point ID for idempotent upsert
                point_id = chunk.metadata.get("content_hash", "")
                if point_id:
                    # Use first 32 hex chars as UUID-like deterministic ID
                    point_id = str(uuid.UUID(point_id[:32]))
                else:
                    point_id = str(uuid.uuid4())

                vectors: dict = {"dense": vec}
                if batch_sparse and i < len(batch_sparse):
                    sp_indices, sp_values = batch_sparse[i]
                    if sp_indices:
                        vectors[SPARSE_VECTOR_NAME] = SparseVector(
                            indices=sp_indices, values=sp_values
                        )

                points.append(PointStruct(
                    id=point_id,
                    vector=vectors,
                    payload={**chunk.metadata, "text": chunk.text},
                ))

            # Retry on connection error
            for attempt in range(MAX_RETRIES):
                try:
                    self.client.upsert(collection_name=self.collection, points=points)
                    break
                except Exception as e:
                    if attempt < MAX_RETRIES - 1:
                        wait = 2 ** attempt
                        print(f"\n[loader] Retry {attempt + 1}/{MAX_RETRIES} after {wait}s: {e}")
                        time.sleep(wait)
                    else:
                        raise RuntimeError(f"Qdrant upsert failed after {MAX_RETRIES} retries: {e}") from e

            uploaded += len(points)
            print(f"[loader] Uploaded {uploaded}/{total} points …", end="\r")

        print(f"\n[loader] Done. {uploaded} points in '{self.collection}'.")

    def collection_info(self) -> dict:
        info = self.client.get_collection(self.collection)
        return {
            "name": self.collection,
            "indexed_vectors_count": info.indexed_vectors_count,
            "points_count": info.points_count,
            "status": str(info.status),
        }
