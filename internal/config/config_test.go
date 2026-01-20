package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		setupFile   bool
		expectError bool
		wantID      string
		wantSecret  string
	}{
		{
			name: "Load from Environment Variables",
			envVars: map[string]string{
				"FFLOG_CLIENT_ID":     "id",
				"FFLOG_CLIENT_SECRET": "secret",
			},
			setupFile:   false,
			expectError: false,
			wantID:      "id",
			wantSecret:  "secret",
		},
		{
			name: "Load from Environment Variable File",
			envVars: map[string]string{
				"FFLOG_CLIENT_ID":     "id",
				"FFLOG_CLIENT_SECRET": "secret",
			},
			setupFile:   true,
			expectError: false,
			wantID:      "id",
			wantSecret:  "secret",
		},
		{
			name: "Missing Config File and Env",
			envVars: map[string]string{
				"FFLOG_CLIENT_ID":     "",
				"FFLOG_CLIENT_SECRET": "",
			},
			setupFile:   false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			if tt.setupFile {
				content := []byte("CLIENT_ID=id\nCLIENT_SECRET=secret")
				err := os.WriteFile(".env", content, 0644)
				assert.NoError(t, err)
				defer func() {
					_ = os.Remove(".env")
				}()
			}

			cfg, err := LoadConfig()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, cfg.ClientID)
				assert.Equal(t, tt.wantSecret, cfg.ClientSecret)
			}
		})
	}
}
