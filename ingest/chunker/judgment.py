"""
judgment.py — Sliding window chunker for judgment prose.

512 tokens window, 64 token overlap (character approximation: *4).
Extracts metadata: case_name, citation, court, date, bench, statutes_cited.
"""

import hashlib
import re
from dataclasses import dataclass, field
from typing import List, Optional

from ingest.chunker.statute import Chunk


@dataclass
class JudgmentMetadata:
    case_name: str = ""
    citation: str = ""
    court: str = ""
    date: str = ""
    bench: str = ""
    statutes_cited: List[str] = field(default_factory=list)

    def to_dict(self) -> dict:
        return {
            "case_name": self.case_name,
            "citation": self.citation,
            "court": self.court,
            "judgment_date": self.date,
            "bench": self.bench,
            "statutes_cited": ",".join(self.statutes_cited),
            "is_judgment": "true",
            "document_type": "judgment",
            "tenant_id": "public",
        }


# Common statute reference patterns for extraction
_STATUTE_REFS = re.compile(
    r"(?:Section|Art(?:icle)?\.?)\s+(\d+[A-Z]?)\s+(?:of\s+)?(?:the\s+)?"
    r"(IPC|BNS|CrPC|BNSS|BSA|Evidence Act|Constitution|Companies Act|"
    r"Income Tax Act|Motor Vehicles Act|Negotiable Instruments Act)",
    re.IGNORECASE,
)


def extract_statutes_cited(text: str) -> List[str]:
    """Extract statute references from judgment text."""
    matches = _STATUTE_REFS.findall(text)
    refs = set()
    for section, statute in matches:
        refs.add(f"{statute} §{section}")
    return sorted(refs)


def chunk_judgment(
    text: str,
    metadata: Optional[JudgmentMetadata] = None,
    window_size: int = 512,
    overlap: int = 64,
) -> List[Chunk]:
    """
    Chunk judgment prose using a sliding window.

    Args:
        text: Full judgment text.
        metadata: Extracted judgment metadata.
        window_size: Window size in approximate tokens (chars / 4).
        overlap: Overlap in approximate tokens.

    Returns:
        List of Chunk objects with metadata.
    """
    if metadata is None:
        metadata = JudgmentMetadata()

    # Approximate tokenization: 1 token ≈ 4 characters
    char_window = window_size * 4
    char_overlap = overlap * 4

    # Extract cited statutes from the full text
    if not metadata.statutes_cited:
        metadata.statutes_cited = extract_statutes_cited(text)

    base_meta = metadata.to_dict()
    chunks = []
    step = char_window - char_overlap

    for i in range(0, max(len(text), 1), step):
        chunk_text = text[i: i + char_window].strip()
        if not chunk_text:
            continue

        chunk_hash = hashlib.sha256(chunk_text.encode("utf-8")).hexdigest()

        chunk_meta = {
            **base_meta,
            "chunk_index": len(chunks),
            "content_hash": chunk_hash,
        }
        chunks.append(Chunk(text=chunk_text, metadata=chunk_meta))

        # Stop if we've covered the entire text
        if i + char_window >= len(text):
            break

    return chunks
