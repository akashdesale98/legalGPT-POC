package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"testqdrant/internal/llm"
	"testqdrant/internal/rag"
)

const maxQueryLen = 2000

// WorkerPool is the subset of worker.Pool used by ChatHandler.
// Defined as an interface to avoid import cycles and ease testing.
type WorkerPool interface {
	Acquire(ctx context.Context) error
	Release()
}

// ChatHandler handles SSE streaming chat requests.
type ChatHandler struct {
	Provider          llm.LLMProvider
	Retriever         *rag.HybridRetriever
	Guardrail         *rag.Guardrail
	Assembler         *rag.ContextAssembler
	IPCDetector       *rag.IPCDetector // optional; enriches prompts with IPC→BNS mappings
	DefaultCollection string
	Pool              WorkerPool // may be nil in tests
}

// ChatRequest is the JSON body for POST /api/v1/chat.
type ChatRequest struct {
	Query       string   `json:"query" binding:"required"`
	Collections []string `json:"collections"`
	SessionID   string   `json:"session_id"`
	Language    string   `json:"language"`
}

// Handle processes a chat request with SSE streaming.
// POST /api/v1/chat
func (h *ChatHandler) Handle(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. Sanitise input
	query, err := sanitizeQuery(req.Query)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// 2. Acquire worker pool slot (backpressure)
	if h.Pool != nil {
		if err := h.Pool.Acquire(ctx); err != nil {
			c.Header("Retry-After", "5")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "server busy",
			})
			return
		}
		defer h.Pool.Release()
	}
	start := time.Now()

	tenantID, _ := c.Get("tenant_id")
	tenant, _ := tenantID.(string)
	if tenant == "" {
		tenant = "public"
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 2. Embed query
	embeddings, err := h.Provider.Embed(ctx, []string{query})
	if err != nil {
		writeSSEError(c.Writer, "embed_failed", err.Error())
		return
	}

	// 3. Retrieve via HybridRetriever
	collection := h.DefaultCollection
	if len(req.Collections) > 0 {
		collection = req.Collections[0]
	}
	chunks, err := h.Retriever.Retrieve(ctx, embeddings[0], query, rag.RetrievalOptions{
		Collection:  collection,
		Collections: req.Collections,
		TopK:        20,
		TenantID:    tenant,
	})
	if err != nil {
		writeSSEError(c.Writer, "retrieval_failed", err.Error())
		return
	}

	// 4. Guardrail check
	guardResult := h.Guardrail.Check(chunks, query)
	if guardResult.ShouldAbstain {
		writeSSEEvent(c.Writer, "abstain", map[string]any{
			"reason":  guardResult.Reason,
			"message": rag.AbstentionMessage(guardResult.Reason),
		})
		writeSSEEvent(c.Writer, "done", map[string]any{
			"abstained": true,
		})
		c.Writer.Flush()
		return
	}

	// 5. Assemble context
	ctxWindow := h.Assembler.BuildContext(chunks)

	// 6. Build messages and stream (with optional IPC→BNS enrichment)
	messages := rag.BuildMessages(query, ctxWindow, h.IPCDetector)
	stream, err := h.Provider.Stream(ctx, llm.CompletionRequest{Messages: messages})
	if err != nil {
		writeSSEError(c.Writer, "llm_failed", err.Error())
		return
	}

	// 7. Forward stream chunks as SSE events — check ctx.Done() between chunks
	var totalOutput int
	chunksSent := 0
	queryHashForLog, _ := c.Get("query_hash") // may be empty, that's fine
	for {
		select {
		case <-ctx.Done():
			log.Printf("SSE: client_disconnected query_hash=%v chunks_sent=%d", queryHashForLog, chunksSent)
			return
		case chunk, ok := <-stream:
			if !ok {
				goto done
			}
			if chunk.Error != nil {
				writeSSEError(c.Writer, "stream_error", chunk.Error.Error())
				return
			}
			if chunk.Done {
				goto done
			}
			writeSSEEvent(c.Writer, "chunk", map[string]any{
				"delta": chunk.Text,
				"done":  false,
			})
			totalOutput += len(chunk.Text)
			chunksSent++
			c.Writer.Flush()
		}
	}
done:

	// 8. Send sources event
	citations := make([]map[string]any, 0, len(ctxWindow.Chunks))
	for _, ch := range ctxWindow.Chunks {
		citation := map[string]any{
			"score": ch.Score,
		}
		if v, ok := ch.Metadata["statute"]; ok {
			citation["statute"] = v
		}
		if v, ok := ch.Metadata["section"]; ok {
			citation["section"] = v
		}
		if v, ok := ch.Metadata["article_number"]; ok {
			citation["article_number"] = v
		}
		if v, ok := ch.Metadata["article_title"]; ok {
			citation["article_title"] = v
		}
		if v, ok := ch.Metadata["source"]; ok {
			citation["source"] = v
		}
		citations = append(citations, citation)
	}
	writeSSEEvent(c.Writer, "sources", map[string]any{
		"citations": citations,
	})

	// 9. Send meta event
	elapsed := time.Since(start)
	writeSSEEvent(c.Writer, "meta", map[string]any{
		"input_tokens":  len(query) / 4,
		"output_tokens": totalOutput / 4,
		"latency_ms":    elapsed.Milliseconds(),
		"provider":      h.Provider.Name(),
	})

	writeSSEEvent(c.Writer, "done", map[string]any{
		"abstained": false,
	})

	c.Writer.Flush()
}

func sanitizeQuery(input string) (string, error) {
	s := strings.ReplaceAll(input, "\x00", "")
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) {
		return "", fmt.Errorf("invalid UTF-8 input")
	}
	if len(s) == 0 {
		return "", fmt.Errorf("query is empty")
	}
	if len(s) > maxQueryLen {
		return "", fmt.Errorf("query too long (%d chars, max %d)", len(s), maxQueryLen)
	}
	return s, nil
}

func writeSSEEvent(w io.Writer, event string, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("SSE marshal error: %v", err)
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(jsonData))
}

func writeSSEError(w io.Writer, code string, message string) {
	writeSSEEvent(w, "error", map[string]any{
		"code":    code,
		"message": message,
	})
}
