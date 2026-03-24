package commentable

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"

	"github.com/gofiber/fiber/v2"
	auth "github.com/nicolasbonnici/gorest-auth"
	rbac "github.com/nicolasbonnici/gorest-rbac"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/query"
)

type CommentHooks struct {
	db         database.Database
	config     *Config
	voter      rbac.Voter
	getComment func(ctx context.Context, id any) (*Comment, error)
}

func NewCommentHooks(db database.Database, config *Config, voter rbac.Voter) *CommentHooks {
	h := &CommentHooks{
		db:     db,
		config: config,
		voter:  voter,
	}
	h.getComment = h.defaultGetComment
	return h
}

func (h *CommentHooks) Create(c *fiber.Ctx, dto CommentCreateDTO, model *Comment) error {
	if !h.config.IsAllowedType(dto.Commentable) {
		return fiber.NewError(400, "commentable type is not allowed")
	}

	user := auth.GetAuthenticatedUser(c)
	if !h.config.AllowAnonymous && user == nil {
		return fiber.NewError(401, "authentication required to comment")
	}

	content := strings.TrimSpace(dto.Content)
	if content == "" {
		return fiber.NewError(400, "content cannot be empty")
	}

	if len(content) > h.config.MaxContentLength {
		return fiber.NewError(400, "content exceeds maximum length")
	}

	model.Content = html.EscapeString(content)

	if user != nil {
		model.UserId = &user.UserID
	}

	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")
	if ipAddress != "" {
		model.IpAddress = &ipAddress
	}
	if userAgent != "" {
		model.UserAgent = &userAgent
	}

	ctx := auth.Context(c)
	tempId := model.Id
	tempUserId := model.UserId
	tempIpAddress := model.IpAddress
	tempUserAgent := model.UserAgent

	// Clear system fields before RBAC validation to prevent tampering
	model.Id = ""
	model.UserId = nil
	model.IpAddress = nil
	model.UserAgent = nil

	if user != nil {
		if err := h.voter.ValidateWrite(ctx, model); err != nil {
			return fiber.NewError(403, fmt.Sprintf("insufficient permissions: %v", err))
		}
	}

	// Restore system fields with trusted values
	model.Id = tempId
	model.UserId = tempUserId
	model.IpAddress = tempIpAddress
	model.UserAgent = tempUserAgent

	return nil
}

func (h *CommentHooks) Update(c *fiber.Ctx, dto CommentUpdateDTO, model *Comment) error {
	if dto.Content == nil && dto.Status == nil {
		return fiber.NewError(400, "at least one field must be provided")
	}

	id := c.Params("id")
	ctx := auth.Context(c)

	existing, err := h.getComment(ctx, id)
	if err != nil {
		return fiber.NewError(404, "Comment not found")
	}

	if err := h.checkOwnership(c, existing); err != nil {
		return err
	}

	if dto.Content != nil {
		sanitized, err := h.validateAndSanitizeContent(*dto.Content)
		if err != nil {
			return err
		}
		model.Content = sanitized
		existing.Content = sanitized
	}

	if dto.Status != nil {
		if err := h.validateStatus(*dto.Status); err != nil {
			return err
		}
		model.Status = *dto.Status
		existing.Status = *dto.Status
	}

	updateItem := *existing
	updateItem.Id = ""
	updateItem.UserId = nil
	updateItem.CreatedAt = nil
	updateItem.UpdatedAt = nil
	updateItem.IpAddress = nil
	updateItem.UserAgent = nil

	if err := h.voter.ValidateWrite(ctx, &updateItem); err != nil {
		return fiber.NewError(403, fmt.Sprintf("insufficient permissions: %v", err))
	}

	return nil
}

func (h *CommentHooks) checkOwnership(c *fiber.Ctx, existing *Comment) error {
	// Anonymous comment - only moderators can edit
	if existing.UserId == nil {
		if h.isModerator(c) {
			return nil
		}
		return fiber.NewError(403, "Only moderators can edit anonymous comments")
	}

	// Authenticated comment - must be owner or moderator
	user := auth.GetAuthenticatedUser(c)
	if user == nil {
		return fiber.NewError(403, "You must be authenticated to edit this comment")
	}

	if *existing.UserId == user.UserID || h.isModerator(c) {
		return nil
	}

	return fiber.NewError(403, "You can only edit your own comments")
}

func (h *CommentHooks) validateAndSanitizeContent(content string) (string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", fiber.NewError(400, "content cannot be empty")
	}

	if len(trimmed) > h.config.MaxContentLength {
		return "", fiber.NewError(400, "content exceeds maximum length")
	}

	return html.EscapeString(trimmed), nil
}

func (h *CommentHooks) validateStatus(status string) error {
	for _, s := range ValidStatuses {
		if status == s {
			return nil
		}
	}
	return fiber.NewError(400, fmt.Sprintf("invalid status value (allowed: %v)", ValidStatuses))
}

func (h *CommentHooks) Delete(c *fiber.Ctx, id any) error {
	ctx := auth.Context(c)

	existing, err := h.getComment(ctx, id)
	if err != nil {
		return fiber.NewError(404, "Comment not found")
	}

	// Anonymous comment - only moderators can delete
	if existing.UserId == nil {
		if h.isModerator(c) {
			return nil
		}
		return fiber.NewError(403, "Only moderators can delete anonymous comments")
	}

	// Authenticated comment - must be owner or moderator
	user := auth.GetAuthenticatedUser(c)
	if user == nil {
		return fiber.NewError(403, "You must be authenticated to delete this comment")
	}

	if *existing.UserId == user.UserID || h.isModerator(c) {
		return nil
	}

	return fiber.NewError(403, "You can only delete your own comments")
}

func (h *CommentHooks) GetByID(c *fiber.Ctx, id any) error {
	return nil
}

func (h *CommentHooks) GetAll(c *fiber.Ctx, conditions *[]query.Condition, orderBy *[]crud.OrderByClause) error {
	if !h.isModerator(c) {
		*conditions = append(*conditions, query.Eq("status", StatusPublished))
	}
	return nil
}

func (h *CommentHooks) isModerator(c *fiber.Ctx) bool {
	roles, ok := rbac.GetRoles(auth.Context(c))
	if !ok || len(roles) == 0 {
		return false
	}
	return h.voter.IsSuperuser(roles) || rbac.HasRole(roles, "moderator", h.voter.GetConfig().RoleHierarchy)
}

func (h *CommentHooks) defaultGetComment(ctx context.Context, id any) (*Comment, error) {
	var comment Comment
	idStr, ok := id.(string)
	if !ok {
		return nil, errors.New("invalid ID type")
	}

	sql := "SELECT * FROM comment WHERE id = " + h.db.Dialect().Placeholder(1)
	err := h.db.QueryRow(ctx, sql, idStr).Scan(
		&comment.Id,
		&comment.UserId,
		&comment.CommentableId,
		&comment.Commentable,
		&comment.ParentId,
		&comment.Content,
		&comment.Status,
		&comment.IpAddress,
		&comment.UserAgent,
		&comment.UpdatedAt,
		&comment.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &comment, nil
}
