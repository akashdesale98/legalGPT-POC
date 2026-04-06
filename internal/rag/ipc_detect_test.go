package rag

import (
	"strings"
	"testing"

	"testqdrant/internal/store"
)

func testMappings() []store.IPCBNSMapping {
	return []store.IPCBNSMapping{
		{IPCSection: "302", BNSSection: "101", Description: "Murder"},
		{IPCSection: "420", BNSSection: "316", Description: "Cheating and dishonestly inducing delivery of property"},
		{IPCSection: "376", BNSSection: "63", Description: "Rape"},
		{IPCSection: "304a", BNSSection: "106", Description: "Causing death by negligence"},
		{IPCSection: "498a", BNSSection: "85", Description: "Cruelty by husband or relatives"},
	}
}

func TestIPCDetector_Detect(t *testing.T) {
	d := NewIPCDetector(testMappings())

	tests := []struct {
		name    string
		query   string
		wantIPC []string // expected IPC sections
	}{
		{
			name:    "section N IPC",
			query:   "What is the punishment under section 302 IPC?",
			wantIPC: []string{"302"},
		},
		{
			name:    "IPC section N",
			query:   "Explain IPC section 420",
			wantIPC: []string{"420"},
		},
		{
			name:    "IPC N",
			query:   "IPC 376 punishment details",
			wantIPC: []string{"376"},
		},
		{
			name:    "N IPC",
			query:   "Is 302 IPC bailable?",
			wantIPC: []string{"302"},
		},
		{
			name:    "section with suffix",
			query:   "What is section 304a IPC?",
			wantIPC: []string{"304a"},
		},
		{
			name:    "Indian Penal Code full name",
			query:   "section 302 of the Indian Penal Code",
			wantIPC: []string{"302"},
		},
		{
			name:    "multiple sections",
			query:   "difference between section 302 IPC and section 420 IPC",
			wantIPC: []string{"302", "420"},
		},
		{
			name:    "no IPC reference",
			query:   "What is Article 21 of the Constitution?",
			wantIPC: nil,
		},
		{
			name:    "BNS reference not IPC",
			query:   "What is section 101 BNS?",
			wantIPC: nil,
		},
		{
			name:    "sec dot abbreviation",
			query:   "sec. 498a IPC explanation",
			wantIPC: []string{"498a"},
		},
		{
			name:    "unknown section",
			query:   "section 999 IPC",
			wantIPC: nil, // no mapping for 999
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := d.Detect(tt.query)
			if len(matches) != len(tt.wantIPC) {
				t.Errorf("got %d matches, want %d", len(matches), len(tt.wantIPC))
				return
			}
			for i, m := range matches {
				if m.IPCSection != tt.wantIPC[i] {
					t.Errorf("match[%d] IPC=%q, want %q", i, m.IPCSection, tt.wantIPC[i])
				}
				if m.BNSSection == "" {
					t.Errorf("match[%d] BNS section is empty", i)
				}
			}
		})
	}
}

func TestIPCDetector_EnrichSystemPrompt(t *testing.T) {
	d := NewIPCDetector(testMappings())

	t.Run("enriches when IPC found", func(t *testing.T) {
		original := "You are a legal AI."
		enriched := d.EnrichSystemPrompt(original, "What is section 302 IPC?")

		if !strings.Contains(enriched, original) {
			t.Error("enriched prompt should contain original")
		}
		if !strings.Contains(enriched, "IPC Section 302") {
			t.Error("should mention IPC Section 302")
		}
		if !strings.Contains(enriched, "BNS Section 101") {
			t.Error("should mention BNS Section 101")
		}
		if !strings.Contains(enriched, "Murder") {
			t.Error("should include description")
		}
	})

	t.Run("no change when no IPC", func(t *testing.T) {
		original := "You are a legal AI."
		enriched := d.EnrichSystemPrompt(original, "What is Article 21?")
		if enriched != original {
			t.Error("should not modify prompt when no IPC references found")
		}
	})
}

func TestIPCDetector_EmptyMappings(t *testing.T) {
	d := NewIPCDetector(nil)
	matches := d.Detect("section 302 IPC")
	if matches != nil {
		t.Error("empty detector should return nil")
	}
}
