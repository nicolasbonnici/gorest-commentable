package commentable

import (
	"time"
)

type CommentCreateDTO struct {
	CommentableId string  `json:"commentableId"`
	Commentable   string  `json:"commentable"`
	ParentId      *string `json:"parentId,omitempty"`
	Content       string  `json:"content"`

	// Anti-spam inputs (used only for anonymous comments when anti-spam is
	// enabled; ignored otherwise). Challenge/Solution carry the proof-of-work
	// token issued by GET /comments/challenge and its computed solution. Website
	// is a honeypot field that must stay empty.
	Challenge string `json:"challenge,omitempty"`
	Solution  string `json:"solution,omitempty"`
	Website   string `json:"website,omitempty"`
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
