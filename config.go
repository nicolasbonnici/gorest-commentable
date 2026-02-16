package commentable

import (
	"errors"
	"fmt"

	"github.com/nicolasbonnici/gorest/database"
)

type Config struct {
	Database           database.Database
	AllowedTypes       []string `json:"allowed_types" yaml:"allowed_types"`
	MaxContentLength   int      `json:"max_content_length" yaml:"max_content_length"`
	PaginationLimit    int      `json:"pagination_limit" yaml:"pagination_limit"`
	MaxPaginationLimit int      `json:"max_pagination_limit" yaml:"max_pagination_limit"`
	EnableNesting      bool     `json:"enable_nesting" yaml:"enable_nesting"`
	MaxNestingDepth    int      `json:"max_nesting_depth" yaml:"max_nesting_depth"`
	DefaultStatus      string   `json:"default_status" yaml:"default_status"`
}

func DefaultConfig() Config {
	return Config{
		AllowedTypes:       []string{"post"},
		MaxContentLength:   10000,
		PaginationLimit:    20,
		MaxPaginationLimit: 100,
		EnableNesting:      true,
		MaxNestingDepth:    10,
		DefaultStatus:      StatusAwaiting,
	}
}

func (c *Config) Validate() error {
	if len(c.AllowedTypes) == 0 {
		return errors.New("allowed_types cannot be empty")
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, commentableType := range c.AllowedTypes {
		if commentableType == "" {
			return errors.New("allowed_types cannot contain empty strings")
		}
		if seen[commentableType] {
			return fmt.Errorf("duplicate type in allowed_types: %s", commentableType)
		}
		seen[commentableType] = true
	}

	if c.MaxContentLength < 1 || c.MaxContentLength > 1048576 {
		return errors.New("max_content_length must be between 1 and 1048576 bytes")
	}

	if c.PaginationLimit < 1 || c.PaginationLimit > c.MaxPaginationLimit {
		return errors.New("pagination_limit must be between 1 and max_pagination_limit")
	}

	if c.MaxNestingDepth < 1 || c.MaxNestingDepth > 100 {
		return errors.New("max_nesting_depth must be between 1 and 100")
	}

	// Validate default status
	if c.DefaultStatus == "" {
		return errors.New("default_status cannot be empty")
	}

	validStatus := false
	for _, status := range ValidStatuses {
		if c.DefaultStatus == status {
			validStatus = true
			break
		}
	}
	if !validStatus {
		return fmt.Errorf("invalid default_status: %s (allowed: %v)", c.DefaultStatus, ValidStatuses)
	}

	return nil
}

func (c *Config) IsAllowedType(commentableType string) bool {
	for _, allowed := range c.AllowedTypes {
		if allowed == commentableType {
			return true
		}
	}
	return false
}
