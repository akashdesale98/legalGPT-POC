"""
run.py — CLI entry point for the ingestion pipeline.

Usage:
    python -m ingest.pipeline.run --source local --collection constitution --file data/constitution.pdf
    python -m ingest.pipeline.run --source local --collection bns --file data/bns.pdf
"""

import argparse
import json
import os
import time
from urllib.parse import urlparse

from ingest.chunker.statute import chunk_pdf
from ingest.embedder.embed import create_embedder
from ingest.store.qdrant_client import QdrantLoader
from ingest.pipeline.config import IngestConfig


def parse_args():
    p = argparse.ArgumentParser(description="Indian Legal GPT — Ingest Pipeline")
    p.add_argument("--source", default="local", choices=["local", "gazette", "kanoon"])
    p.add_argument("--collection", default="constitution")
    p.add_argument("--file", default="../data/constitution.pdf", help="Path to PDF file")
    p.add_argument("--source-name", default="constitution_of_india")
    p.add_argument("--doc-type", default="constitutional_provision")
    p.add_argument("--act-name", default="Constitution of India")
    p.add_argument("--mode", default="auto", choices=["auto", "constitution", "pages"])
    p.add_argument("--reset", action="store_true")
    p.add_argument("--dry-run", action="store_true")
    p.add_argument("--ipc-bns-map", default=None, help="Path to IPC-BNS mapping JSON")
    return p.parse_args()


def load_ipc_bns_map(path):
    """Load IPC→BNS mapping from JSON file."""
    if not path or not os.path.exists(path):
        return None
    with open(path) as f:
        data = json.load(f)
    # Build dict: BNS section → IPC section description
    mapping = {}
    for entry in data:
        if entry.get("bns_section") and entry["bns_section"] != "null":
            mapping[entry["bns_section"]] = f"IPC §{entry['ipc_section']} ({entry['description']})"
    return mapping


def main():
    args = parse_args()
    cfg = IngestConfig()

    print("=" * 60)
    print("  Indian Legal GPT — Ingest Pipeline")
    print("=" * 60)
    print(f"  Source     : {args.source}")
    print(f"  File       : {args.file}")
    print(f"  Collection : {args.collection}")
    print(f"  Mode       : {args.mode}")
    print(f"  Provider   : {cfg.embedding_provider}")
    print(f"  Dry run    : {args.dry_run}")
    print("=" * 60)

    start_time = time.time()

    # Load IPC→BNS mapping if provided
    ipc_bns_map = load_ipc_bns_map(args.ipc_bns_map)
    if ipc_bns_map:
        print(f"[ingest] Loaded {len(ipc_bns_map)} IPC→BNS mappings")

    # Step 1: Chunk
    chunks = chunk_pdf(
        pdf_path=args.file,
        source=args.source_name,
        document_type=args.doc_type,
        mode=args.mode,
        act_name=args.act_name,
        ipc_bns_map=ipc_bns_map,
    )

    if not chunks:
        print("[ingest] ERROR: No chunks produced. Check PDF path and format.")
        sys.exit(1)

    print(f"\n[ingest] Sample chunk:")
    c = chunks[0]
    print(f"  Article  : {c.metadata.get('article_number', 'N/A')}")
    print(f"  Title    : {c.metadata.get('article_title', 'N/A')}")
    print(f"  Part     : {c.metadata.get('part', 'N/A')}")
    print(f"  Chapter  : {c.metadata.get('chapter', 'N/A')}")
    print(f"  Hash     : {c.metadata.get('content_hash', 'N/A')[:16]}…")
    print(f"  Preview  : {c.text[:150]} …\n")

    if args.dry_run:
        print(f"[ingest] Dry-run: skipping embedding and Qdrant load.")
        print(f"[ingest] Total chunks that would be loaded: {len(chunks)}")
        return

    # Step 2: Embed
    print("[ingest] Embedding chunks …")
    embedder = create_embedder(
        provider=cfg.embedding_provider,
        ollama_url=cfg.ollama_url,
        model=cfg.embed_model,
        cohere_api_key=cfg.cohere_api_key,
    )
    texts = [c.text for c in chunks]
    embeddings = embedder.embed_batch(texts, show_progress=True)

    # Validate dimensions
    actual_size = len(embeddings[0])
    print(f"[ingest] Embedding dimensions: {actual_size}")

    # Step 3: Load into Qdrant
    parsed_url = urlparse(cfg.qdrant_url)
    qdrant_host = parsed_url.hostname or "localhost"
    qdrant_port = parsed_url.port or 6333

    print(f"\n[ingest] Loading into Qdrant ({qdrant_host}:{qdrant_port}) …")
    loader = QdrantLoader(
        host=qdrant_host,
        port=qdrant_port,
        collection=args.collection,
        vector_size=actual_size,
        api_key=cfg.qdrant_api_key,
    )
    loader.ensure_collection(reset=args.reset)
    loader.load(chunks, embeddings)

    # Step 4: Quality check
    try:
        from ingest.quality.gates import post_ingest_check
        result = post_ingest_check(loader.client, args.collection, expected_min_count=len(chunks) // 2)
        if not result.passed:
            print(f"[ingest] WARNING: Quality check failed: {result.message}")
    except ImportError:
        print("[ingest] Skipping quality check (module not available)")

    # Done
    info = loader.collection_info()
    elapsed = time.time() - start_time
    print(f"\n[ingest] Collection info:")
    print(json.dumps(info, indent=2))
    print(f"\n[ingest] Pipeline complete in {elapsed:.1f}s.")


if __name__ == "__main__":
    main()
