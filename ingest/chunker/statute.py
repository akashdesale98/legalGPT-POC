"""
statute.py — Parse and chunk legal PDF documents with Summary-Augmented Chunking (SAC).

Primary strategy  : Hierarchical SAC chunking (for Indian Constitution, BNS, BNSS, BSA).
Fallback strategy : Page-level chunking (for any other legal PDF).

Each chunk carries rich metadata including hierarchical context (Part/Chapter/Section).
Parent summary is prepended to each chunk per ARCHITECTURE.md §3.1.
"""

import hashlib
import json
import re
from dataclasses import dataclass, field
from typing import List, Optional

try:
    import fitz  # PyMuPDF
except ImportError:
    fitz = None

try:
    import pdfplumber
except ImportError:
    pdfplumber = None


@dataclass
class Chunk:
    text: str
    metadata: dict = field(default_factory=dict)


# ---------------------------------------------------------------------------
# PDF text extraction (preserved from POC, with pdfplumber fallback)
# ---------------------------------------------------------------------------

def extract_text(pdf_path: str) -> str:
    """Extract text from PDF. Primary: PyMuPDF; fallback: pdfplumber."""
    if fitz is not None:
        with fitz.open(pdf_path) as doc:
            pages = [str(page.get_text("text")) for page in doc]
        return "\n".join(pages)
    elif pdfplumber is not None:
        with pdfplumber.open(pdf_path) as pdf:
            pages = [page.extract_text() or "" for page in pdf.pages]
        return "\n".join(pages)
    else:
        raise ImportError(
            "Neither PyMuPDF nor pdfplumber is installed. "
            "Install one: pip install PyMuPDF pdfplumber"
        )


def content_hash(text: str) -> str:
    """SHA-256 hash of text for deduplication."""
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


# ---------------------------------------------------------------------------
# Regex patterns (preserved and extended from POC chunker.py)
# ---------------------------------------------------------------------------

_PART_RE = re.compile(
    r"^\s*PART\s+([IVXLCDM]+[A-Z]?)\s*$",
    re.IGNORECASE,
)

_ARTICLE_RE = re.compile(
    r"^\s*(\d{1,3}[A-Z]?)\.\s+([^—\-\n]{3,}?)(?:[—\-]+|\.—|\s*$)",
)

_SCHEDULE_RE = re.compile(
    r"^\s*(FIRST|SECOND|THIRD|FOURTH|FIFTH|SIXTH|SEVENTH|EIGHTH|NINTH|TENTH|ELEVENTH|TWELFTH)\s+SCHEDULE\b",
    re.IGNORECASE,
)

_CHAPTER_RE = re.compile(
    r"^\s*CHAPTER\s+([IVXLCDM]+[A-Z]?)\s*[-—]?\s*(.*)$",
    re.IGNORECASE,
)

MAX_CHUNK_WORDS = 400


# ---------------------------------------------------------------------------
# SAC helpers
# ---------------------------------------------------------------------------

def _build_parent_summary(
    act_name: str, part: str, chapter: str,
    article_num: str, article_title: str,
) -> str:
    """Build parent summary to prepend per SAC specification."""
    lines = [f"Act: {act_name}"]
    if part:
        lines.append(f"Part: {part}")
    if chapter:
        lines.append(f"Chapter: {chapter}")
    if article_num:
        lines.append(f"Section {article_num}: {article_title}")
    return "\n".join(lines)


def _flush_sac(
    chunks: List[Chunk],
    act_name: str, part: str, chapter: str,
    article_num: str, article_title: str,
    lines: List[str], source: str, document_type: str,
    effective_date: str = "", superseded_by: str = "",
    ipc_bns_map: Optional[dict] = None,
):
    """Flush buffered lines into SAC chunks with parent summary prepended."""
    if not lines or not article_num:
        return

    full_text = " ".join(" ".join(lines).split())
    if not full_text.strip():
        return

    parent_summary = _build_parent_summary(act_name, part, chapter, article_num, article_title)
    text_hash = content_hash(full_text)

    ipc_equivalent = ""
    if ipc_bns_map and article_num in ipc_bns_map:
        ipc_equivalent = ipc_bns_map[article_num]

    base_metadata = {
        "source": source,
        "document_type": document_type,
        "part": part,
        "chapter": chapter,
        "article_number": article_num,
        "article_title": article_title,
        "content_hash": text_hash,
        "effective_date": effective_date,
        "superseded_by": superseded_by,
        "tenant_id": "public",
    }
    if ipc_equivalent:
        base_metadata["ipc_equivalent"] = ipc_equivalent

    words = full_text.split()
    for idx in range(0, max(len(words), 1), MAX_CHUNK_WORDS):
        chunk_text = " ".join(words[idx: idx + MAX_CHUNK_WORDS])
        if not chunk_text.strip():
            continue

        sac_text = f"[PARENT SUMMARY]\n{parent_summary}\n\n[CHUNK CONTENT]\n{chunk_text}"
        metadata = {**base_metadata, "chunk_index": idx // MAX_CHUNK_WORDS}
        chunks.append(Chunk(text=sac_text, metadata=metadata))


# ---------------------------------------------------------------------------
# Constitution parser (preserved core logic, extended with SAC + chapters)
# ---------------------------------------------------------------------------

def parse_constitution(
    text: str,
    source: str = "constitution_of_india",
    document_type: str = "constitutional_provision",
    act_name: str = "Constitution of India",
    ipc_bns_map: Optional[dict] = None,
) -> List[Chunk]:
    """Parse Indian Constitution/statute text into SAC article-level chunks."""
    chunks: List[Chunk] = []
    current_part = "Preamble"
    current_chapter = ""
    current_article_num = ""
    current_article_title = ""
    buffer: List[str] = []
    in_schedule = False

    for raw_line in text.splitlines():
        line = raw_line.strip()
        if not line:
            continue

        part_m = _PART_RE.match(line)
        if part_m:
            _flush_sac(chunks, act_name, current_part, current_chapter,
                       current_article_num, current_article_title, buffer,
                       source, document_type, ipc_bns_map=ipc_bns_map)
            buffer = []
            current_part = f"Part {part_m.group(1).upper()}"
            in_schedule = False
            continue

        chap_m = _CHAPTER_RE.match(line)
        if chap_m:
            _flush_sac(chunks, act_name, current_part, current_chapter,
                       current_article_num, current_article_title, buffer,
                       source, document_type, ipc_bns_map=ipc_bns_map)
            buffer = []
            current_chapter = f"Chapter {chap_m.group(1).upper()}"
            if chap_m.group(2).strip():
                current_chapter += f" — {chap_m.group(2).strip()}"
            continue

        sched_m = _SCHEDULE_RE.match(line)
        if sched_m:
            _flush_sac(chunks, act_name, current_part, current_chapter,
                       current_article_num, current_article_title, buffer,
                       source, document_type, ipc_bns_map=ipc_bns_map)
            buffer = []
            current_part = sched_m.group(0).strip().title()
            current_article_num = ""
            current_article_title = ""
            in_schedule = True
            continue

        if in_schedule:
            buffer.append(line)
            continue

        art_m = _ARTICLE_RE.match(line)
        if art_m:
            _flush_sac(chunks, act_name, current_part, current_chapter,
                       current_article_num, current_article_title, buffer,
                       source, document_type, ipc_bns_map=ipc_bns_map)
            buffer = []
            current_article_num = art_m.group(1)
            current_article_title = art_m.group(2).strip().rstrip(".")
            remainder = line[art_m.end():].strip().lstrip("—-").strip()
            if remainder:
                buffer.append(remainder)
            continue

        if current_article_num:
            buffer.append(line)

    _flush_sac(chunks, act_name, current_part, current_chapter,
               current_article_num, current_article_title, buffer,
               source, document_type, ipc_bns_map=ipc_bns_map)

    return chunks


# ---------------------------------------------------------------------------
# Generic page-level fallback chunker (preserved from POC)
# ---------------------------------------------------------------------------

def chunk_by_pages(
    text: str,
    source: str = "unknown",
    document_type: str = "legal_document",
    words_per_chunk: int = 400,
) -> List[Chunk]:
    """Generic fallback: split raw text into fixed-size word windows."""
    words = text.split()
    chunks = []
    for idx in range(0, len(words), words_per_chunk):
        chunk_text = " ".join(words[idx: idx + words_per_chunk])
        chunks.append(Chunk(
            text=chunk_text,
            metadata={
                "source": source,
                "document_type": document_type,
                "chunk_index": idx // words_per_chunk,
                "content_hash": content_hash(chunk_text),
                "tenant_id": "public",
            },
        ))
    return chunks


# ---------------------------------------------------------------------------
# Public entry point (preserved from POC, extended)
# ---------------------------------------------------------------------------

def chunk_pdf(
    pdf_path: str,
    source: str = "constitution_of_india",
    document_type: str = "constitutional_provision",
    mode: str = "auto",
    act_name: str = "Constitution of India",
    ipc_bns_map: Optional[dict] = None,
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
        chunks = parse_constitution(text, source, document_type, act_name, ipc_bns_map)
    else:
        chunks = parse_constitution(text, source, document_type, act_name, ipc_bns_map)
        if len(chunks) < 10:
            print("[chunker] Article parsing found < 10 chunks — falling back to page-level chunking")
            chunks = chunk_by_pages(text, source, document_type)

    print(f"[chunker] Produced {len(chunks)} chunks")
    return chunks


if __name__ == "__main__":
    import sys
    pdf = sys.argv[1] if len(sys.argv) > 1 else "../../data/constitution.pdf"
    chunks = chunk_pdf(pdf)
    print(f"\nSample (first 3 chunks):")
    for c in chunks[:3]:
        print(json.dumps({"metadata": c.metadata, "text_preview": c.text[:200]}, indent=2))
