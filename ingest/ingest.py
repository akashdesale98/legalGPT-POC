"""
ingest.py — Full ingestion pipeline runner.

Usage:
    python ingest.py [OPTIONS]

Options:
    --pdf PATH          Path to the PDF file  [default: ../data/constitution.pdf]
    --collection NAME   Qdrant collection name [default: constitution]
    --source NAME       Source identifier in metadata [default: constitution_of_india]
    --doc-type TYPE     Document type in metadata [default: constitutional_provision]
    --mode MODE         Chunking mode: auto|constitution|pages [default: auto]
    --reset             Drop and recreate the Qdrant collection before loading
    --qdrant-host HOST  Qdrant host [default: localhost]
    --qdrant-port PORT  Qdrant port [default: 6333]
    --ollama-url URL    Ollama base URL [default: http://localhost:11434]
    --embed-model MODEL Ollama embedding model [default: nomic-embed-text]
    --vector-size SIZE  Embedding vector dimensions [default: 768]
    --dry-run           Chunk and embed only; do not write to Qdrant

Example:
    python ingest.py --pdf ../data/constitution.pdf --reset
"""

import argparse
import json
import sys

from chunker import chunk_pdf
from embedder import OllamaEmbedder
from qdrant_loader import QdrantLoader


def parse_args():
    p = argparse.ArgumentParser(description="Indian Legal GPT — Ingest Pipeline")
    p.add_argument("--pdf",          default="../data/constitution.pdf")
    p.add_argument("--collection",   default="constitution")
    p.add_argument("--source",       default="constitution_of_india")
    p.add_argument("--doc-type",     default="constitutional_provision")
    p.add_argument("--mode",         default="auto", choices=["auto", "constitution", "pages"])
    p.add_argument("--reset",        action="store_true")
    p.add_argument("--qdrant-host",  default="localhost")
    p.add_argument("--qdrant-port",  type=int, default=6333)
    p.add_argument("--ollama-url",   default="http://localhost:11434")
    p.add_argument("--embed-model",  default="nomic-embed-text")
    p.add_argument("--vector-size",  type=int, default=768)
    p.add_argument("--dry-run",      action="store_true")
    return p.parse_args()


def main():
    args = parse_args()

    print("=" * 60)
    print("  Indian Legal GPT — Ingest Pipeline")
    print("=" * 60)
    print(f"  PDF        : {args.pdf}")
    print(f"  Collection : {args.collection}")
    print(f"  Mode       : {args.mode}")
    print(f"  Embed model: {args.embed_model}")
    print(f"  Dry run    : {args.dry_run}")
    print("=" * 60)

    # --- Step 1: Chunk ---
    chunks = chunk_pdf(
        pdf_path=args.pdf,
        source=args.source,
        document_type=args.doc_type,
        mode=args.mode,
    )

    if not chunks:
        print("[ingest] ERROR: No chunks produced. Check PDF path and format.")
        sys.exit(1)

    print(f"\n[ingest] Sample chunk:")
    c = chunks[0]
    print(f"  Article  : {c.metadata.get('article_number', 'N/A')}")
    print(f"  Title    : {c.metadata.get('article_title', 'N/A')}")
    print(f"  Part     : {c.metadata.get('part', 'N/A')}")
    print(f"  Preview  : {c.text[:150]} …\n")

    if args.dry_run:
        print(f"[ingest] Dry-run: skipping embedding and Qdrant load.")
        print(f"[ingest] Total chunks that would be loaded: {len(chunks)}")
        return

    # --- Step 2: Embed ---
    print("[ingest] Embedding chunks via Ollama …")
    embedder = OllamaEmbedder(base_url=args.ollama_url, model=args.embed_model)
    texts = [c.text for c in chunks]
    embeddings = embedder.embed_batch(texts, show_progress=True)

    # Validate vector size
    actual_size = len(embeddings[0])
    if actual_size != args.vector_size:
        print(
            f"[ingest] WARNING: expected vector size {args.vector_size}, "
            f"got {actual_size}. Updating config."
        )
        args.vector_size = actual_size

    # --- Step 3: Load into Qdrant ---
    print(f"\n[ingest] Loading into Qdrant ({args.qdrant_host}:{args.qdrant_port}) …")
    loader = QdrantLoader(
        host=args.qdrant_host,
        port=args.qdrant_port,
        collection=args.collection,
        vector_size=args.vector_size,
    )
    loader.ensure_collection(reset=args.reset)
    loader.load(chunks, embeddings)

    # --- Done ---
    info = loader.collection_info()
    print("\n[ingest] Collection info:")
    print(json.dumps(info, indent=2))
    print("\n[ingest] Pipeline complete.")


if __name__ == "__main__":
    main()
