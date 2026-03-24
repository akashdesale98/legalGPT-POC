"""
gates.py — Post-ingest quality gates.

Verifies that ingested data meets minimum quality standards.
"""

import random
from dataclasses import dataclass
from typing import Optional

from qdrant_client import QdrantClient


@dataclass
class QualityResult:
    passed: bool
    collection: str
    point_count: int
    expected_min: int
    sample_metadata_complete: bool
    message: str


def post_ingest_check(
    client: QdrantClient,
    collection: str,
    expected_min_count: int = 10,
) -> QualityResult:
    """
    Verify collection quality after ingestion.

    Checks:
    - Collection exists
    - Point count >= expected_min_count
    - Sample 10 random points have populated metadata
    """
    try:
        info = client.get_collection(collection)
    except Exception as e:
        return QualityResult(
            passed=False,
            collection=collection,
            point_count=0,
            expected_min=expected_min_count,
            sample_metadata_complete=False,
            message=f"Collection does not exist: {e}",
        )

    point_count = info.points_count or 0

    if point_count < expected_min_count:
        return QualityResult(
            passed=False,
            collection=collection,
            point_count=point_count,
            expected_min=expected_min_count,
            sample_metadata_complete=False,
            message=f"Point count {point_count} < expected minimum {expected_min_count}",
        )

    # Sample 10 random points and verify metadata
    sample_size = min(10, point_count)
    try:
        # Use random offset to avoid always sampling the same first N points
        max_offset = max(0, point_count - sample_size)
        offset = random.randint(0, max_offset) if max_offset > 0 else 0
        results = client.scroll(
            collection_name=collection,
            limit=sample_size,
            offset=offset,
            with_payload=True,
        )
        points = results[0] if results else []
    except Exception as e:
        return QualityResult(
            passed=False,
            collection=collection,
            point_count=point_count,
            expected_min=expected_min_count,
            sample_metadata_complete=False,
            message=f"Failed to sample points: {e}",
        )

    required_fields = {"text", "source", "document_type", "tenant_id"}
    metadata_complete = True
    for point in points:
        payload = point.payload or {}
        missing = required_fields - set(payload.keys())
        if missing:
            metadata_complete = False
            print(f"[quality] Point {point.id} missing fields: {missing}")

    passed = metadata_complete
    message = "PASS" if passed else "FAIL: some points missing required metadata fields"

    result = QualityResult(
        passed=passed,
        collection=collection,
        point_count=point_count,
        expected_min=expected_min_count,
        sample_metadata_complete=metadata_complete,
        message=message,
    )

    status = "PASS" if result.passed else "FAIL"
    print(f"[quality] {status}: collection={collection} points={point_count} metadata_ok={metadata_complete}")

    return result
