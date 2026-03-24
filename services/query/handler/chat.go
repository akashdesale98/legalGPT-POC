package handler

import (
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

// ChatHandler handles SSE streaming chat requests.
type ChatHandler struct {
	Provider          llm.LLMProvider
	Retriever         *rag.HybridRetriever
	Guardrail         *rag.Guardrail
	Assembler         *rag.ContextAssembler
	DefaultCollection string
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

	// 6. Build messages and stream
	messages := rag.BuildMessages(query, ctxWindow)
	stream, err := h.Provider.Stream(ctx, llm.CompletionRequest{Messages: messages})
	if err != nil {
		writeSSEError(c.Writer, "llm_failed", err.Error())
		return
	}

	// 7. Forward stream chunks as SSE events
	var totalOutput int
	for chunk := range stream {
		if chunk.Error != nil {
			writeSSEError(c.Writer, "stream_error", chunk.Error.Error())
			return
		}
		if chunk.Done {
			break
		}
		writeSSEEvent(c.Writer, "chunk", map[string]any{
			"delta": chunk.Text,
			"done":  false,
		})
		totalOutput += len(chunk.Text)
		c.Writer.Flush()
	}

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
