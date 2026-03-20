package commentable

import (
	"time"
)

type CommentCreateDTO struct {
	CommentableId string  `json:"commentableId"`
	Commentable   string  `json:"commentable"`
	ParentId      *string `json:"parentId,omitempty"`
	Content       string  `json:"content"`
}

type CommentUpdateDTO struct {
	Content *string `json:"content,omitempty"`
	Status  *string `json:"status,omitempty"`
}

type CommentResponseDTO struct {
	ID            string     `json:"id"`
	UserID        *string    `json:"userId,omitempty"`
	CommentableID string     `json:"commentableId"`
	Commentable   string     `json:"commentable"`
	ParentID      *string    `json:"parentId,omitempty"`
	Content       string     `json:"content"`
	Status        string     `json:"status"`
	IPAddress     *string    `json:"ipAddress,omitempty"`
	UserAgent     *string    `json:"userAgent,omitempty"`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
	CreatedAt     *time.Time `json:"createdAt,omitempty"`
}
