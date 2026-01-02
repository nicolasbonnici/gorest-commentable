package commentable

import (
	"errors"
	"html"
	"strings"
	"time"
)

type Comment struct {
	Id            string     `json:"id,omitempty" db:"id"`
	UserId        *string    `json:"userId,omitempty" db:"user_id"`
	CommentableId string     `json:"commentableId" db:"commentable_id"`
	Commentable   string     `json:"commentable" db:"commentable"`
	ParentId      *string    `json:"parentId,omitempty" db:"parent_id"`
	Content       string     `json:"content" db:"content"`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty" db:"updated_at"`
	CreatedAt     *time.Time `json:"createdAt,omitempty" db:"created_at"`
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
	Content string `json:"content" validate:"required"`
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

	// Sanitize HTML to prevent XSS
	r.Content = html.EscapeString(r.Content)

	return nil
}

func (r *UpdateCommentRequest) Validate(config *Config) error {
	r.Content = strings.TrimSpace(r.Content)
	if r.Content == "" {
		return errors.New("content cannot be empty")
	}

	if len(r.Content) > config.MaxContentLength {
		return errors.New("content exceeds maximum length")
	}

	// Sanitize HTML to prevent XSS
	r.Content = html.EscapeString(r.Content)

	return nil
}
