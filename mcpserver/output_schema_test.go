package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeOutputSchema(t *testing.T) {
	s := sanitizeOutputSchema(mustSchema[APIResult]())
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), `"body":true`) {
		t.Fatalf("boolean schema node survived: %s", raw)
	}
	if !strings.Contains(string(raw), `"body":{"description":`) {
		t.Fatalf("body schema not replaced: %s", raw)
	}
}
