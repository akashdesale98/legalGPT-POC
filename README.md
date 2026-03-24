# Indian Legal GPT — POC

A RAG-based legal AI system for Indian law, starting with the **Constitution of India**.
uilt with Qdrant (vector DB), Ollama (local LLM), Go (interactive CLI), and Python (ingest pipeline).

---

## Architecture

```
constitution.pdf
      │
      ▼
[Python Ingest Pipeline]
  chunker.py       →  Article-level chunks + metadata
  embedder.py      →  Ollama nomic-embed-text embeddings
  qdrant_loader.py →  Upsert into Qdrant collection
      │
      ▼
[Qdrant Vector DB]  (localhost:6333 REST / 6334 gRPC)
      │
      ▼
[Go Interactive CLI]
  >> user question  →  embed query → search Qdrant → call Ollama → print answer + sources
      │
      ▼
[Ollama LLM]        (localhost:11434)
  nomic-embed-text  →  embeddings
  llama3.2          →  answer generation
```

---

## Prerequisites

| Tool | Install |
|------|---------|
| Docker | https://docs.docker.com/get-docker/ |
| Ollama | https://ollama.com/download |
| Python 3.10+ | https://www.python.org/downloads/ |
| Go 1.21+ | https://go.dev/dl/ |

---

## Step 1 — Start Qdrant

```bash
docker run -d --name qdrant -p 6333:6333 -p 6334:6334 qdrant/qdrant
```

Verify: open http://localhost:6333/dashboard in your browser.

---

## Step 2 — Pull Ollama Models

```bash
# Embedding model (768-dim vectors)
ollama pull nomic-embed-text

# Chat / generation model
ollama pull llama3.2
```

> **Tip:** `llama3.2` is ~2 GB. If you want a smaller model, edit `config.json`
> and change `"chat_model"` to `"tinyllama"` or any other model you have pulled.

---

## Step 3 — Ingest the Constitution

```bash
cd ingest
pip install -r requirements.txt
python ingest.py --reset
```

Expected output:

```
============================================================
  Indian Legal GPT — Ingest Pipeline
============================================================
  PDF        : ../data/constitution.pdf
  Collection : constitution
  Mode       : auto
  Embed model: nomic-embed-text
  Dry run    : False
============================================================
[chunker] Extracting text from ../data/constitution.pdf …
[chunker] Extracted 412,xxx characters
[chunker] Produced 4xx chunks

[ingest] Sample chunk:
  Article  : 21
  Title    : Protection of life and personal liberty
  Part     : Part III
  Preview  : No person shall be deprived of his life or personal liberty …

[embedder] 450/450 embedded …
[loader] Creating collection 'constitution' (size=768, distance=COSINE) …
[loader] Done. 450 points in 'constitution'.
```

### Ingest CLI options

| Flag | Default | Description |
|------|---------|-------------|
| `--pdf` | `../data/constitution.pdf` | Path to input PDF |
| `--collection` | `constitution` | Qdrant collection name |
| `--source` | `constitution_of_india` | Metadata source tag |
| `--doc-type` | `constitutional_provision` | Metadata document type |
| `--mode` | `auto` | `auto` / `constitution` / `pages` |
| `--reset` | off | Drop & recreate collection before loading |
| `--dry-run` | off | Parse + embed only; skip Qdrant write |
| `--embed-model` | `nomic-embed-text` | Ollama embedding model |
| `--chat-model` | `llama3.2` | (config.json) LLM model |
| `--qdrant-host` | `localhost` | Qdrant host |
| `--qdrant-port` | `6333` | Qdrant port |
| `--ollama-url` | `http://localhost:11434` | Ollama base URL |

---

## Step 4 — Build the CLI Tool

```bash
# From the project root
go build -o legal-gpt.exe .
```

---

## Step 5 — Ask Questions (Interactive Mode)

```bash
.\legal-gpt.exe
```

Expected output:

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Indian Legal GPT — Interactive CLI
  Qdrant     : localhost:6334
  Ollama     : http://localhost:11434
  Embed      : nomic-embed-text
  Chat       : llama3.2
  Collection : constitution
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Type your question and press Enter.
  Type 'exit' or 'quit' to stop.

>>
```

### Sample questions to try

```
>> What are the fundamental rights of a citizen?
>> What does Article 21 say about right to life?
>> What are the restrictions on freedom of speech?
>> Explain the Directive Principles of State Policy
>> What is the right to equality under the Constitution?
```

Each answer includes cited sources with article numbers and response latency:

```
>> What is Article 32?

Article 32 provides the right to move the Supreme Court for enforcement
of the fundamental rights conferred by Part III of the Constitution...

  Sources:
    [1] Article 32 — Remedies for enforcement of rights (score: 0.91)
    [2] Article 226 — Power of High Courts to issue certain writs (score: 0.78)

  (2.4s)
```

---

## Configuration

Edit `config.json` in the project root to change any default:

```json
{
  "qdrant_host": "localhost",
  "qdrant_port": 6334,
  "ollama_url": "http://localhost:11434",
  "embed_model": "nomic-embed-text",
  "chat_model": "llama3.2",
  "vector_size": 768,
  "default_top_k": 5,
  "default_collection": "constitution"
}
```

---

## Adding More Legal Documents (Generic Usage)

The system is designed to be portable. To add IPC, CrPC, or any other statute:

```bash
# 1. Ingest the new document into its own collection
cd ingest
python ingest.py \
  --pdf ../data/ipc.pdf \
  --collection ipc \
  --source ipc_1860 \
  --doc-type penal_code \
  --reset

# 2. Update config.json to point to the new collection
#    "default_collection": "ipc"
# 3. Run the CLI
.\legal-gpt.exe
>> What is the punishment for theft?
```

---

## Project Structure

```
testQdrant/
├── config.json          # Runtime config (models, ports)
├── main.go              # Interactive CLI — REPL loop
├── config.go            # Config loader
├── ollama.go            # Ollama embed + chat client
├── rag.go               # Vector search + RAG prompt builder
├── go.mod / go.sum
├── data/
│   └── constitution.pdf
└── ingest/
    ├── chunker.py        # PDF → structured article chunks
    ├── embedder.py       # Ollama embedding wrapper
    ├── qdrant_loader.py  # Qdrant upsert helper
    ├── ingest.py         # Pipeline CLI runner
    ├── pdf_parser.py     # Original constitution parser
    └── requirements.txt
```