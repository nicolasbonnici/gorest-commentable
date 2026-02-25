package commentable

import (
	"errors"
	"fmt"
	"html"
	"strings"
	"time"
)

const (
	StatusAwaiting  = "awaiting"
	StatusPublished = "published"
	StatusDraft     = "draft"
	StatusModerated = "moderated"
)

var ValidStatuses = []string{
	StatusAwaiting,
	StatusPublished,
	StatusDraft,
	StatusModerated,
}

type Comment struct {
	Id            string     `json:"id,omitempty" db:"id" rbac:"read:*;write:none"`
	UserId        *string    `json:"userId,omitempty" db:"user_id" rbac:"read:*;write:reader"`
	CommentableId string     `json:"commentableId" db:"commentable_id" rbac:"read:*;write:reader"`
	Commentable   string     `json:"commentable" db:"commentable" rbac:"read:*;write:reader"`
	ParentId      *string    `json:"parentId,omitempty" db:"parent_id" rbac:"read:*;write:reader"`
	Content       string     `json:"content" db:"content" rbac:"read:*;write:reader"`
	Status        string     `json:"status" db:"status" rbac:"read:*;write:moderator"`
	IpAddress     *string    `json:"ipAddress,omitempty" db:"ip_address" rbac:"read:moderator;write:none"`
	UserAgent     *string    `json:"userAgent,omitempty" db:"user_agent" rbac:"read:moderator;write:none"`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty" db:"updated_at" rbac:"read:*;write:none"`
	CreatedAt     *time.Time `json:"createdAt,omitempty" db:"created_at" rbac:"read:*;write:none"`
}

func (Comment) TableName() string {
	return "comment"
}

type CreateCommentRequest struct {
	CommentableId string  `json:"commentableId" validate:"required,uuid"`
	Commentable   string  `json:"commentable" validate:"required"`
	ParentId      *string `json:"parentId,omitempty" validate:"omitempty,uuid"`
	Content       string  `json:"content" validate:"required"`
}

type UpdateCommentRequest struct {
	Content *string `json:"content,omitempty"`
	Status  *string `json:"status,omitempty"`
}

func (r *CreateCommentRequest) Validate(config *Config) error {
	if !config.IsAllowedType(r.Commentable) {
		return errors.New("commentable type is not allowed")
	}

	r.Content = strings.TrimSpace(r.Content)
	if r.Content == "" {
		return errors.New("content cannot be empty")
	}

	if len(r.Content) > config.MaxContentLength {
		return errors.New("content exceeds maximum length")
	}

	r.Content = html.EscapeString(r.Content)

	return nil
}

func (r *UpdateCommentRequest) Validate(config *Config) error {
	// At least one field must be provided
	if r.Content == nil && r.Status == nil {
		return errors.New("at least one field must be provided")
	}

	if r.Content != nil {
		trimmed := strings.TrimSpace(*r.Content)
		if trimmed == "" {
			return errors.New("content cannot be empty")
		}

		if len(trimmed) > config.MaxContentLength {
			return errors.New("content exceeds maximum length")
		}

		sanitized := html.EscapeString(trimmed)
		r.Content = &sanitized
	}

	if r.Status != nil {
		valid := false
		for _, s := range ValidStatuses {
			if *r.Status == s {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid status value (allowed: %v)", ValidStatuses)
		}
	}

	return nil
}
