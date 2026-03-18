package commentable

import (
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
