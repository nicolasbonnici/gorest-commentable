package commentable

import (
	"testing"

	"github.com/nicolasbonnici/gorest/filter"
)

func TestCommentResource_validateCommentableFilter(t *testing.T) {
	config := &Config{
		AllowedTypes: []string{"post", "article", "product"},
	}

	resource := &CommentResource{
		Config: config,
	}

	tests := []struct {
		name        string
		filterField string
		values      []string
		expectError bool
	}{
		{
			name:        "valid single commentable type",
			filterField: "commentable",
			values:      []string{"post"},
			expectError: false,
		},
		{
			name:        "valid multiple commentable types",
			filterField: "commentable",
			values:      []string{"post", "article"},
			expectError: false,
		},
		{
			name:        "invalid commentable type",
			filterField: "commentable",
			values:      []string{"invalid"},
			expectError: true,
		},
		{
			name:        "mixed valid and invalid types",
			filterField: "commentable",
			values:      []string{"post", "invalid"},
			expectError: true,
		},
		{
			name:        "non-commentable field",
			filterField: "user_id",
			values:      []string{"some-uuid"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters := &filter.FilterSet{
				Filters: []filter.Filter{
					{
						Field:    tt.filterField,
						Operator: filter.OpIn,
						Values:   tt.values,
					},
				},
				AllowedFields: map[string]bool{
					"commentable": true,
					"user_id":     true,
				},
			}

			filters.AllowedFields = map[string]bool{
				tt.filterField: true,
			}

			err := resource.validateCommentableFilter(filters)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCommentResource_validateFilterLimits(t *testing.T) {
	resource := &CommentResource{}

	tests := []struct {
		name        string
		valueCount  int
		expectError bool
	}{
		{
			name:        "within limit - 1 value",
			valueCount:  1,
			expectError: false,
		},
		{
			name:        "within limit - 50 values",
			valueCount:  50,
			expectError: false,
		},
		{
			name:        "exceeds limit - 51 values",
			valueCount:  51,
			expectError: true,
		},
		{
			name:        "exceeds limit - 100 values",
			valueCount:  100,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := make([]string, tt.valueCount)
			for i := 0; i < tt.valueCount; i++ {
				values[i] = "value"
			}

			filters := &filter.FilterSet{
				Filters: []filter.Filter{
					{
						Field:    "commentable",
						Operator: filter.OpIn,
						Values:   values,
					},
				},
				AllowedFields: map[string]bool{
					"commentable": true,
				},
			}

			err := resource.validateFilterLimits(filters)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
