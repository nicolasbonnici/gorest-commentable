package commentable

import (
	"testing"
)

func TestConfig_ValidateDefaultStatus(t *testing.T) {
	tests := []struct {
		name          string
		defaultStatus string
		wantErr       bool
		errContains   string
	}{
		{
			name:          "valid status awaiting",
			defaultStatus: StatusAwaiting,
			wantErr:       false,
		},
		{
			name:          "valid status published",
			defaultStatus: StatusPublished,
			wantErr:       false,
		},
		{
			name:          "valid status draft",
			defaultStatus: StatusDraft,
			wantErr:       false,
		},
		{
			name:          "valid status moderated",
			defaultStatus: StatusModerated,
			wantErr:       false,
		},
		{
			name:          "invalid status",
			defaultStatus: "invalid",
			wantErr:       true,
			errContains:   "invalid default_status",
		},
		{
			name:          "empty status",
			defaultStatus: "",
			wantErr:       true,
			errContains:   "default_status cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := DefaultConfig()
			c.DefaultStatus = tt.defaultStatus

			err := c.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Config.Validate() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}

func TestDefaultConfig_HasAwaitingStatus(t *testing.T) {
	config := DefaultConfig()
	if config.DefaultStatus != StatusAwaiting {
		t.Errorf("DefaultConfig() DefaultStatus = %v, want %v", config.DefaultStatus, StatusAwaiting)
	}

	// Verify default config is valid
	if err := config.Validate(); err != nil {
		t.Errorf("DefaultConfig() should be valid, got error: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
