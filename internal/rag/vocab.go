package rag

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"unicode"
)

// sparseVocab is the in-memory representation of the BM25 vocabulary
// produced by ingest/embedder/sparse.py.
type sparseVocab struct {
	TermToID map[string]int     `json:"term_to_id"`
	DF       map[string]int     `json:"df"` // keys are stringified ints
	NumDocs  int                `json:"num_docs"`
}

var (
	vocabCache   = map[string]*sparseVocab{}
	vocabCacheMu sync.RWMutex
)

// loadVocab reads the JSON vocab file for tenantID.
// Results are cached per path for the lifetime of the process.
func loadVocab(path string) (*sparseVocab, error) {
	vocabCacheMu.RLock()
	if v, ok := vocabCache[path]; ok {
		vocabCacheMu.RUnlock()
		return v, nil
	}
	vocabCacheMu.RUnlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("vocab load %s: %w", path, err)
	}

	var v sparseVocab
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("vocab parse %s: %w", path, err)
	}

	vocabCacheMu.Lock()
	vocabCache[path] = &v
	vocabCacheMu.Unlock()
	return &v, nil
}

// encodeQuery converts a query string into (indices, values) using the vocab.
// Mirrors the TF-IDF logic in sparse.py: tf * (log((N+1)/(df+1)) + 1).
func (v *sparseVocab) encodeQuery(query string) ([]uint32, []float32) {
	tokens := tokenizeGo(query)
	if len(tokens) == 0 {
		return nil, nil
	}

	tf := map[int]int{}
	for _, t := range tokens {
		if id, ok := v.TermToID[t]; ok {
			tf[id]++
		}
	}
	if len(tf) == 0 {
		return nil, nil
	}

	nDocs := v.NumDocs
	if nDocs < 1 {
		nDocs = 1
	}

	indices := make([]uint32, 0, len(tf))
	values := make([]float32, 0, len(tf))
	for termID, freq := range tf {
		dfVal := 1
		if d, ok := v.DF[fmt.Sprintf("%d", termID)]; ok {
			dfVal = d
		}
		idf := math.Log(float64(nDocs+1)/float64(dfVal+1)) + 1.0
		indices = append(indices, uint32(termID))
		values = append(values, float32(freq)*float32(idf))
	}
	return indices, values
}

// stopwords mirrors the Python set in sparse.py.
var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true,
	"of": true, "in": true, "to": true, "for": true, "with": true,
	"on": true, "at": true, "from": true, "by": true, "about": true,
	"as": true, "into": true, "through": true,
	"and": true, "or": true, "but": true, "not": true,
	"this": true, "that": true, "which": true, "who": true, "whom": true,
	"under": true, "section": true, "act": true, "any": true, "every": true,
	"such": true, "where": true, "when": true,
}

func tokenizeGo(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var cur strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else {
			if cur.Len() > 1 {
				t := cur.String()
				if !stopwords[t] {
					tokens = append(tokens, t)
				}
			}
			cur.Reset()
		}
	}
	if cur.Len() > 1 {
		t := cur.String()
		if !stopwords[t] {
			tokens = append(tokens, t)
		}
	}
	return tokens
}
