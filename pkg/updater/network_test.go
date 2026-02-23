package updater

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestCheckForUpdates_Network(t *testing.T) {
	// Assume current version is v0.9.2 from version.go (hardcoded knowledge, but acceptable for unit tests)
	// Better: we can't easily mock version.Version without changing that package or doing link-time substitution.
	// Instead, we'll construct scenarios based on whatever version.Version is, assuming it's valid.

	tests := []struct {
		name           string
		responseBody   string
		responseStatus int
		expectTag      string
		expectURL      string
		expectErr      bool
	}{
		{
			name:           "Newer version available",
			responseBody:   `{"tag_name": "v99.0.0", "html_url": "http://example.com/release"}`,
			responseStatus: http.StatusOK,
			expectTag:      "v99.0.0",
			expectURL:      "http://example.com/release",
			expectErr:      false,
		},
		{
			name:           "Same version (no update)",
			responseBody:   `{"tag_name": "v0.0.0", "html_url": "http://example.com/release"}`, // Assumes current > v0.0.0
			responseStatus: http.StatusOK,
			expectTag:      "",
			expectURL:      "",
			expectErr:      false,
		},
		{
			name:           "Rate limit (403)",
			responseBody:   `{"message": "rate limit exceeded"}`,
			responseStatus: http.StatusForbidden,
			expectTag:      "",
			expectURL:      "",
			expectErr:      false, // Should swallow error
		},
		{
			name:           "Server error (500)",
			responseBody:   "",
			responseStatus: http.StatusInternalServerError,
			expectTag:      "",
			expectURL:      "",
			expectErr:      true,
		},
		{
			name:           "Invalid JSON",
			responseBody:   `{invalid json}`,
			responseStatus: http.StatusOK,
			expectTag:      "",
			expectURL:      "",
			expectErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := server.Client()
			client.Timeout = 1 * time.Second

			tag, url, err := checkForUpdates(client, server.URL)

			if (err != nil) != tt.expectErr {
				t.Errorf("checkForUpdates() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if tag != tt.expectTag {
				t.Errorf("checkForUpdates() tag = %v, want %v", tag, tt.expectTag)
			}
			if url != tt.expectURL {
				t.Errorf("checkForUpdates() url = %v, want %v", url, tt.expectURL)
			}
		})
	}
}

// TestCheckForUpdates_GitHubTokenAuth verifies that GITHUB_TOKEN is sent as a
// Bearer token in the Authorization header when set (#117).
func TestCheckForUpdates_GitHubTokenAuth(t *testing.T) {
	tests := []struct {
		name      string
		envVar    string
		envVal    string
		wantAuth  string
	}{
		{"GITHUB_TOKEN set", "GITHUB_TOKEN", "ghp_test123", "Bearer ghp_test123"},
		{"GH_TOKEN set", "GH_TOKEN", "gho_fallback456", "Bearer gho_fallback456"},
		{"No token set", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear both env vars first
			os.Unsetenv("GITHUB_TOKEN")
			os.Unsetenv("GH_TOKEN")
			if tt.envVar != "" {
				os.Setenv(tt.envVar, tt.envVal)
				defer os.Unsetenv(tt.envVar)
			}

			var gotAuth string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"tag_name": "v0.0.1", "html_url": "http://example.com"}`))
			}))
			defer server.Close()

			client := server.Client()
			client.Timeout = 1 * time.Second
			checkForUpdates(client, server.URL)

			if gotAuth != tt.wantAuth {
				t.Errorf("Authorization header = %q, want %q", gotAuth, tt.wantAuth)
			}
		})
	}
}

// TestGitHubToken_Precedence verifies GITHUB_TOKEN takes precedence over GH_TOKEN.
func TestGitHubToken_Precedence(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "primary")
	os.Setenv("GH_TOKEN", "fallback")
	defer os.Unsetenv("GITHUB_TOKEN")
	defer os.Unsetenv("GH_TOKEN")

	tok := githubToken()
	if tok != "primary" {
		t.Errorf("githubToken() = %q, want %q (GITHUB_TOKEN should take precedence)", tok, "primary")
	}
}
