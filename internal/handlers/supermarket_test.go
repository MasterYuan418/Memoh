package handlers

import "testing"

func TestMcpEntryToUpsertExpandsTemplateVars(t *testing.T) {
	handler := &SupermarketHandler{}

	upsert := handler.mcpEntryToUpsert(SupermarketMcpEntry{
		Name:      "openai-api",
		Transport: "http",
		URL:       "https://api.openai.com/v1/mcp?key=${OPENAI_API_KEY}",
		Headers: []SupermarketConfigVar{
			{Key: "Authorization", DefaultValue: "Bearer ${OPENAI_API_KEY}"},
			{Key: "X-Missing", DefaultValue: "${MISSING_VAR}"},
		},
		Env: []SupermarketConfigVar{
			{Key: "OPENAI_API_KEY", DefaultValue: ""},
		},
	}, map[string]string{"OPENAI_API_KEY": "sk-test"})

	if upsert.URL != "https://api.openai.com/v1/mcp?key=sk-test" {
		t.Fatalf("unexpected URL: %q", upsert.URL)
	}
	if got := upsert.Headers["Authorization"]; got != "Bearer sk-test" {
		t.Fatalf("unexpected Authorization header: %q", got)
	}
	if got := upsert.Headers["X-Missing"]; got != "${MISSING_VAR}" {
		t.Fatalf("missing variables should be preserved, got %q", got)
	}
}
