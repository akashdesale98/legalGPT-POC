"""
chunker.py — Parse and chunk legal PDF documents.

Primary strategy  : Article-level chunking (for Indian Constitution).
Fallback strategy : Page-level chunking (for any other legal PDF).

Each chunk carries rich metadata so the Go API can cite exact provisions.
"""

import re
import fitz  # PyMuPDF
from dataclasses import dataclass, field
from typing import List, Optional


@dataclass
class Chunk:
    text: str
    metadata: dict = field(default_factory=dict)


# ---------------------------------------------------------------------------
# PDF text extraction
# ---------------------------------------------------------------------------

def extract_text(pdf_path: str) -> str:
    doc = fitz.open(pdf_path)
    pages = [page.get_text("text") for page in doc]
    return "\n".join(pages)


# ---------------------------------------------------------------------------
# Constitution-specific parser
# ---------------------------------------------------------------------------

# Matches: "PART I", "PART XIV", "PART IXA" etc.
_PART_RE = re.compile(
    r"^\s*PART\s+([IVXLCDM]+[A-Z]?)\s*$",
    re.IGNORECASE,
)

# Matches article lines like:
#   "1. Name and territory of the Union.—"
#   "21A. Right to education.—"
#   "370. Temporary provisions with respect to the State of Jammu and Kashmir.—"
_ARTICLE_RE = re.compile(
    r"^\s*(\d{1,3}[A-Z]?)\.\s+([^—\-\n]{3,}?)(?:[—\-]+|\.—|\s*$)",
)

# Schedule headers
_SCHEDULE_RE = re.compile(r"^\s*(FIRST|SECOND|THIRD|FOURTH|FIFTH|SIXTH|SEVENTH|EIGHTH|NINTH|TENTH|ELEVENTH|TWELFTH)\s+SCHEDULE\b", re.IGNORECASE)

MAX_CHUNK_WORDS = 400  # split very long articles into sub-chunks


def _flush(
    chunks: List[Chunk],
    part: str,
    article_num: str,
    article_title: str,
    lines: List[str],
    source: str,
    document_type: str,
):
    """Flush buffered lines into one or more Chunk objects."""
    if not lines or not article_num:
        return

    full_text = " ".join(" ".join(lines).split())  # normalise whitespace
    if not full_text.strip():
        return

    words = full_text.split()
    for idx in range(0, max(len(words), 1), MAX_CHUNK_WORDS):
        chunk_text = " ".join(words[idx : idx + MAX_CHUNK_WORDS])
        if not chunk_text.strip():
            continue
        chunks.append(
            Chunk(
                text=chunk_text,
                metadata={
                    "source": source,
                    "document_type": document_type,
                    "part": part,
                    "article_number": article_num,
                    "article_title": article_title,
                    "chunk_index": idx // MAX_CHUNK_WORDS,
                },
            )
        )


def parse_constitution(
    text: str,
    source: str = "constitution_of_india",
    document_type: str = "constitutional_provision",
) -> List[Chunk]:
    """
    Parse Indian Constitution text into article-level chunks.
    Returns list of Chunk objects with metadata.
    """
    chunks: List[Chunk] = []
    current_part = "Preamble"
    current_article_num = ""
    current_article_title = ""
    buffer: List[str] = []
    in_schedule = False

    for raw_line in text.splitlines():
        line = raw_line.strip()
        if not line:
            continue

        # --- Part header ---
        part_m = _PART_RE.match(line)
        if part_m:
            _flush(chunks, current_part, current_article_num, current_article_title, buffer, source, document_type)
            buffer = []
            current_part = f"Part {part_m.group(1).upper()}"
            in_schedule = False
            continue

        # --- Schedule header ---
        sched_m = _SCHEDULE_RE.match(line)
        if sched_m:
            _flush(chunks, current_part, current_article_num, current_article_title, buffer, source, document_type)
            buffer = []
            current_part = sched_m.group(0).strip().title()
            current_article_num = ""
            current_article_title = ""
            in_schedule = True
            continue

        if in_schedule:
            # Schedules: treat each schedule as a single big chunk
            buffer.append(line)
            continue

        # --- Article header ---
        art_m = _ARTICLE_RE.match(line)
        if art_m:
            _flush(chunks, current_part, current_article_num, current_article_title, buffer, source, document_type)
            buffer = []
            current_article_num = art_m.group(1)
            current_article_title = art_m.group(2).strip().rstrip(".")
            # Remainder of the line after the dash is article text
            remainder = line[art_m.end():].strip().lstrip("—-").strip()
            if remainder:
                buffer.append(remainder)
            continue

        # --- Body text ---
        if current_article_num:
            buffer.append(line)

    # Flush last article
    _flush(chunks, current_part, current_article_num, current_article_title, buffer, source, document_type)

    return chunks


# ---------------------------------------------------------------------------
# Generic page-level fallback chunker
# ---------------------------------------------------------------------------

def chunk_by_pages(
    text: str,
    source: str = "unknown",
    document_type: str = "legal_document",
    words_per_chunk: int = 400,
) -> List[Chunk]:
    """
    Generic fallback: split raw text into fixed-size word windows.
    Used when document structure cannot be detected automatically.
    """
    words = text.split()
    chunks = []
    for idx in range(0, len(words), words_per_chunk):
        chunk_text = " ".join(words[idx : idx + words_per_chunk])
        chunks.append(
            Chunk(
                text=chunk_text,
                metadata={
                    "source": source,
                    "document_type": document_type,
                    "chunk_index": idx // words_per_chunk,
                },
            )
        )
    return chunks


# ---------------------------------------------------------------------------
# Public entry point
# ---------------------------------------------------------------------------

def chunk_pdf(
    pdf_path: str,
    source: str = "constitution_of_india",
    document_type: str = "constitutional_provision",
    mode: str = "auto",  # "auto" | "constitution" | "pages"
) -> List[Chunk]:
    """
    Extract and chunk a legal PDF.

    mode="auto"         : try constitution parser; fall back to pages
    mode="constitution" : force constitution parser
    mode="pages"        : force page-level chunking
    """
    print(f"[chunker] Extracting text from {pdf_path} …")
    text = extract_text(pdf_path)
    print(f"[chunker] Extracted {len(text):,} characters")

    if mode == "pages":
        chunks = chunk_by_pages(text, source, document_type)
    elif mode == "constitution":
        chunks = parse_constitution(text, source, document_type)
    else:  # auto
        chunks = parse_constitution(text, source, document_type)
        if len(chunks) < 10:
            print("[chunker] Article parsing found < 10 chunks — falling back to page-level chunking")
            chunks = chunk_by_pages(text, source, document_type)

    print(f"[chunker] Produced {len(chunks)} chunks")
    return chunks


# ---------------------------------------------------------------------------
# CLI for quick inspection
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    import sys, json

    pdf = sys.argv[1] if len(sys.argv) > 1 else "../data/constitution.pdf"
    chunks = chunk_pdf(pdf)
    print(f"\nSample (first 3 chunks):")
    for c in chunks[:3]:
        print(json.dumps({"metadata": c.metadata, "text_preview": c.text[:200]}, indent=2))
