package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractClientTokenFromRequest(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*http.Request)
		want  string
	}{
		{
			name: "bearer header",
			setup: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer agent-token")
			},
			want: "agent-token",
		},
		{
			name: "legacy v2 query token",
			setup: func(r *http.Request) {
				r.URL.RawQuery = "token=agent-token"
			},
			want: "agent-token",
		},
		{
			name: "query token outside v2 is rejected",
			setup: func(r *http.Request) {
				r.URL.Path = "/api/rpc2"
				r.URL.RawQuery = "token=agent-token"
			},
		},
		{
			name: "body token is rejected",
			setup: func(r *http.Request) {
				r.Header.Set("Content-Type", "application/json")
				r.Body = httptest.NewRequest("POST", "/", strings.NewReader(`{"token":"agent-token"}`)).Body
			},
		},
		{
			name: "basic auth is rejected",
			setup: func(r *http.Request) {
				r.Header.Set("Authorization", "Basic agent-token")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/clients/v2/rpc", nil)
			tt.setup(r)
			if got := extractClientTokenFromRequest(r); got != tt.want {
				t.Fatalf("token = %q, want %q", got, tt.want)
			}
		})
	}
}
