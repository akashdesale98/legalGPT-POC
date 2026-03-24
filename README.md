# Indian Legal GPT — POC

A RAG-based legal AI system for Indian law, starting with the **Constitution of India**.
Built with Qdrant (vector DB), Ollama (local LLM), Go (API), and Python (ingest pipeline).

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
[Qdrant Vector DB]  (localhost:6333)
      │
      ▼
[Go API Server]     (localhost:8080)
  POST /api/query  →  embed query → search Qdrant → call Ollama → return answer + sources
  GET  /health
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
docker run -d --name qdrant -p 6333:6333 qdrant/qdrant
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

## Step 4 — Start the Go API

```bash
# From the project root
go run .
```

Expected output:

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Indian Legal GPT API
  Listening : http://localhost:8080
  Qdrant    : localhost:6333
  Ollama    : http://localhost:11434
  Embed     : nomic-embed-text  |  Chat: llama3.2
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

---

## Step 5 — Query the API

### Health check

```bash
curl http://localhost:8080/health
```

```json
{"service": "indian-legal-gpt", "status": "ok"}
```

### Ask a constitutional question

```bash
curl -s -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What are the fundamental rights of a citizen?"}' | jq .
```

### Sample questions to try

```bash
# Right to life
curl -s -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What does Article 21 say about right to life?"}' | jq .answer

# Freedom of speech
curl -s -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What are the restrictions on freedom of speech?"}' | jq .answer

# UPSC exam question style
curl -s -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{"query": "Explain the Directive Principles of State Policy"}' | jq .answer

# Right to equality
curl -s -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What is the right to equality under the Constitution?"}' | jq .answer
```

### Query request body

```json
{
  "query": "What is Article 32?",
  "collection": "constitution",
  "top_k": 5,
  "model": "llama3.2"
}
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `query` | yes | — | The legal question |
| `collection` | no | `constitution` | Qdrant collection to search |
| `top_k` | no | `5` | Number of chunks to retrieve |
| `model` | no | `llama3.2` | Ollama model to use for generation |

### Query response body

```json
{
  "query": "What does Article 21 say?",
  "answer": "Article 21 guarantees the Protection of life and personal liberty...",
  "sources": [
    {
      "text": "No person shall be deprived of his life or personal liberty...",
      "article_number": "21",
      "article_title": "Protection of life and personal liberty",
      "part": "Part III",
      "source": "constitution_of_india",
      "document_type": "constitutional_provision",
      "score": 0.91
    }
  ]
}
```

---

## Configuration

Edit `config.json` in the project root to change any default:

```json
{
  "qdrant_host": "localhost",
  "qdrant_port": 6333,
  "ollama_url": "http://localhost:11434",
  "embed_model": "nomic-embed-text",
  "chat_model": "llama3.2",
  "vector_size": 768,
  "api_port": 8080,
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

# 2. Query it by passing the collection name
curl -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What is the punishment for theft?", "collection": "ipc"}'
```

---

## Project Structure

```
testQdrant/
├── config.json          # Runtime config (models, ports)
├── main.go              # HTTP server — routes & handlers
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

---

## Troubleshooting

**`ollama: connection refused`**
→ Make sure Ollama is running: `ollama serve`

**`qdrant: connection refused`**
→ Check Docker container: `docker ps` and `docker start qdrant`

**`ollama returned empty embedding`**
→ Model not pulled: `ollama pull nomic-embed-text`

**Chunker produces < 10 chunks**
→ The PDF may use a different format. Try: `python ingest.py --mode pages`

**Build error on `go run .`**
→ Run `go mod tidy` first to sync dependencies.

---

## Roadmap

- [ ] Phase 1 (now): Constitution of India RAG — POC
- [ ] Phase 2: Add metadata filters (act, section, court), hybrid BM25 + vector retrieval
- [ ] Phase 3: IPC, CrPC, Evidence Act ingestion
- [ ] Phase 4: API-first platform with Go + Python SDKs, multilingual support
