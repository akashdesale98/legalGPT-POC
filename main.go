package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

func main() {
	cfg, err := LoadConfig("config.json")
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// Qdrant client
	qdrantClient, err := qdrant.NewClient(&qdrant.Config{
		Host: cfg.QdrantHost,
		Port: cfg.QdrantPort,
	})
	if err != nil {
		log.Fatalf("qdrant client: %v", err)
	}

	// Ollama client
	ollamaClient := NewOllamaClient(cfg.OllamaURL)

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  Indian Legal GPT — Interactive CLI")
	fmt.Printf("  Qdrant     : %s:%d\n", cfg.QdrantHost, cfg.QdrantPort)
	fmt.Printf("  Ollama     : %s\n", cfg.OllamaURL)
	fmt.Printf("  Embed      : %s\n", cfg.EmbedModel)
	fmt.Printf("  Chat       : %s\n", cfg.ChatModel)
	fmt.Printf("  Collection : %s\n", cfg.DefaultCollection)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  Type your question and press Enter.")
	fmt.Println("  Type 'exit' or 'quit' to stop.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(">> ")
		if !scanner.Scan() {
			break
		}

		query := strings.TrimSpace(scanner.Text())
		if query == "" {
			continue
		}
		if query == "exit" || query == "quit" {
			fmt.Println("Bye!")
			return
		}

		start := time.Now()

		// 1. Embed the query
		vector, err := ollamaClient.Embed(cfg.EmbedModel, query)
		if err != nil {
			fmt.Printf("  [error] embedding failed: %v\n\n", err)
			continue
		}

		// 2. Retrieve from Qdrant
		results, err := SearchQdrant(qdrantClient, cfg.DefaultCollection, vector, cfg.DefaultTopK)
		if err != nil {
			fmt.Printf("  [error] retrieval failed: %v\n\n", err)
			continue
		}

		// 3. Generate answer via Ollama
		messages := BuildRAGPrompt(query, results)
		answer, err := ollamaClient.Chat(cfg.ChatModel, messages)
		if err != nil {
			fmt.Printf("  [error] generation failed: %v\n\n", err)
			continue
		}

		elapsed := time.Since(start)

		// Print answer
		fmt.Println()
		fmt.Println(answer)

		// Print sources
		fmt.Println()
		fmt.Println("  Sources:")
		for i, s := range results {
			fmt.Printf("    [%d] Article %s — %s (score: %.2f)\n", i+1, s.ArticleNumber, s.ArticleTitle, s.Score)
		}
		fmt.Printf("\n  (%s)\n\n", elapsed.Round(time.Millisecond))
	}
}
