package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAIConnectionUsesConfiguredModelAndReportsFailures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		success bool
	}{
		{"success", 200, `{"choices":[{"message":{"content":"OK"}}]}`, true},
		{"unauthorized", 401, `secret-body`, false},
		{"empty", 200, `{"choices":[{"message":{"content":""}}]}`, false},
		{"invalid", 200, `{}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-only" {
					t.Error("wrong endpoint or authorization")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := NewAIClient(AISettings{Enabled: true, BaseURL: server.URL + "/v1", Model: "test", APIKey: "test-only"}, server.Client())
			err := client.TestConnection(context.Background())
			if (err == nil) != tc.success {
				t.Fatalf("unexpected test outcome: %v", err)
			}
			if err != nil {
				message := AIErrorMessage(err)
				if strings.Contains(message, server.URL) || strings.Contains(message, "secret-body") || strings.Contains(message, "test-only") {
					t.Fatal("sensitive error detail exposed")
				}
			}
		})
	}
}

func TestAISettingsValidation(t *testing.T) {
	for _, address := range []string{"", "file:///tmp/a", "https://user:password@example.com/v1", "https://example.com/v1?key=secret"} {
		if ValidateAISettings(AISettings{BaseURL: address, Model: "m"}) == nil {
			t.Fatal("accepted invalid address")
		}
	}
	if err := ValidateAISettings(AISettings{BaseURL: "http://localhost:11434/v1", Model: "local"}); err != nil {
		t.Fatal(err)
	}
}
