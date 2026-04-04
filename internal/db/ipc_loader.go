package db

import (
	"encoding/json"
	"fmt"
	"os"

	"testqdrant/internal/store"
)

type ipcBNSRaw struct {
	IPCSection    string `json:"ipc_section"`
	BNSSection    string `json:"bns_section"`
	Description   string `json:"description"`
	EffectiveDate string `json:"effective_date"`
}

// LoadIPCMappings reads the JSON file at path and returns typed mappings.
func LoadIPCMappings(path string) ([]store.IPCBNSMapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ipc_loader: read %s: %w", path, err)
	}

	var raw []ipcBNSRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("ipc_loader: parse: %w", err)
	}

	mappings := make([]store.IPCBNSMapping, 0, len(raw))
	for _, r := range raw {
		mappings = append(mappings, store.IPCBNSMapping{
			IPCSection:    r.IPCSection,
			BNSSection:    r.BNSSection,
			Description:   r.Description,
			EffectiveDate: r.EffectiveDate,
		})
	}
	return mappings, nil
}
