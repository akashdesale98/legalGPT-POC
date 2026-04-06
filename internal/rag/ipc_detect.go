package rag

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"testqdrant/internal/store"
)

// IPCDetector identifies references to old IPC (Indian Penal Code) sections
// in user queries and maps them to their BNS (Bharatiya Nyaya Sanhita) equivalents.
type IPCDetector struct {
	mappings map[string]store.IPCBNSMapping // keyed by normalised IPC section
}

// NewIPCDetector creates an IPCDetector from the loaded IPC→BNS mappings.
func NewIPCDetector(mappings []store.IPCBNSMapping) *IPCDetector {
	m := make(map[string]store.IPCBNSMapping, len(mappings))
	for _, mapping := range mappings {
		key := normalizeSection(mapping.IPCSection)
		m[key] = mapping
	}
	return &IPCDetector{mappings: m}
}

// ipcPatterns matches references to IPC sections in natural language.
// Examples: "section 302 IPC", "IPC 420", "s. 302 of IPC", "sec 376 ipc",
// "under section 302 of the Indian Penal Code", "IPC section 302"
var ipcPatterns = []*regexp.Regexp{
	// "section 302 IPC", "section 302 of IPC", "section 302 of the IPC"
	regexp.MustCompile(`(?i)(?:section|sec\.?|s\.?)\s*(\d+[a-zA-Z]?)\s+(?:of\s+(?:the\s+)?)?(?:IPC|Indian\s+Penal\s+Code)`),
	// "IPC section 302", "IPC sec. 302", "IPC s. 302"
	regexp.MustCompile(`(?i)(?:IPC|Indian\s+Penal\s+Code)\s+(?:section|sec\.?|s\.?)\s*(\d+[a-zA-Z]?)`),
	// "IPC 302", "IPC-302"
	regexp.MustCompile(`(?i)(?:IPC|Indian\s+Penal\s+Code)[\s\-]+(\d+[a-zA-Z]?)`),
	// "302 IPC" (number followed by IPC)
	regexp.MustCompile(`(?i)\b(\d+[a-zA-Z]?)\s+(?:IPC|Indian\s+Penal\s+Code)\b`),
}

// IPCMatch represents a detected IPC reference and its BNS equivalent.
type IPCMatch struct {
	IPCSection  string
	BNSSection  string
	Description string
}

// Detect scans the query for IPC section references and returns matches
// with their BNS equivalents. Returns nil if no IPC references are found.
func (d *IPCDetector) Detect(query string) []IPCMatch {
	if len(d.mappings) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var matches []IPCMatch

	for _, pattern := range ipcPatterns {
		for _, submatch := range pattern.FindAllStringSubmatch(query, -1) {
			if len(submatch) < 2 {
				continue
			}
			section := normalizeSection(submatch[1])
			if seen[section] {
				continue
			}
			seen[section] = true

			if mapping, ok := d.mappings[section]; ok {
				matches = append(matches, IPCMatch{
					IPCSection:  mapping.IPCSection,
					BNSSection:  mapping.BNSSection,
					Description: mapping.Description,
				})
			}
		}
	}

	if len(matches) > 0 {
		sections := make([]string, len(matches))
		for i, m := range matches {
			sections[i] = fmt.Sprintf("IPC %s→BNS %s", m.IPCSection, m.BNSSection)
		}
		log.Printf("IPC_DETECT: matches=%d sections=%s", len(matches), strings.Join(sections, ", "))
	}
	return matches
}

// EnrichSystemPrompt appends IPC→BNS mapping context to the system prompt
// when IPC references are detected in the query.
func (d *IPCDetector) EnrichSystemPrompt(systemPrompt, query string) string {
	matches := d.Detect(query)
	if len(matches) == 0 {
		return systemPrompt
	}

	var sb strings.Builder
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\nIMPORTANT — IPC to BNS Cross-Reference:\n")
	sb.WriteString("The user's query references the Indian Penal Code (IPC), which has been replaced by the Bharatiya Nyaya Sanhita (BNS) 2023. ")
	sb.WriteString("Use the following mapping and answer using the current BNS provisions while noting the old IPC equivalents:\n")
	for _, m := range matches {
		fmt.Fprintf(&sb, "- IPC Section %s → BNS Section %s", m.IPCSection, m.BNSSection)
		if m.Description != "" {
			fmt.Fprintf(&sb, " (%s)", m.Description)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// normalizeSection lowercases and trims whitespace from a section number.
func normalizeSection(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}
