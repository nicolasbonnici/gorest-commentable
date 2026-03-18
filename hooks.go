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
	db     database.Database
	config *Config
	voter  rbac.Voter
}

func NewCommentHooks(db database.Database, config *Config, voter rbac.Voter) *CommentHooks {
	return &CommentHooks{
		db:     db,
		config: config,
		voter:  voter,
	}
}

func (h *CommentHooks) CreateHook(c *fiber.Ctx, dto CommentCreateDTO, model *Comment) error {
	if !h.config.IsAllowedType(dto.Commentable) {
		return fiber.NewError(400, "commentable type is not allowed")
	}

	content := strings.TrimSpace(dto.Content)
	if content == "" {
		return fiber.NewError(400, "content cannot be empty")
	}

	if len(content) > h.config.MaxContentLength {
		return fiber.NewError(400, "content exceeds maximum length")
	}

	model.Content = html.EscapeString(content)

	user := auth.GetAuthenticatedUser(c)
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
	model.Id = ""
	model.UserId = nil
	model.IpAddress = nil
	model.UserAgent = nil

	if err := h.voter.ValidateWrite(ctx, model); err != nil {
		return fiber.NewError(403, fmt.Sprintf("insufficient permissions: %v", err))
	}

	model.Id = tempId
	model.UserId = tempUserId
	if ipAddress != "" {
		model.IpAddress = &ipAddress
	}
	if userAgent != "" {
		model.UserAgent = &userAgent
	}

	return nil
}

func (h *CommentHooks) UpdateHook(c *fiber.Ctx, dto CommentUpdateDTO, model *Comment) error {
	if dto.Content == nil && dto.Status == nil {
		return fiber.NewError(400, "at least one field must be provided")
	}

	id := c.Params("id")
	ctx := auth.Context(c)

	existing, err := h.getComment(ctx, id)
	if err != nil {
		return fiber.NewError(404, "Comment not found")
	}

	user := auth.GetAuthenticatedUser(c)
	if user != nil && existing.UserId != nil && *existing.UserId != user.UserID {
		if !h.isModerator(c) {
			return fiber.NewError(403, "You can only edit your own comments")
		}
	}

	if dto.Content != nil {
		trimmed := strings.TrimSpace(*dto.Content)
		if trimmed == "" {
			return fiber.NewError(400, "content cannot be empty")
		}

		if len(trimmed) > h.config.MaxContentLength {
			return fiber.NewError(400, "content exceeds maximum length")
		}

		sanitized := html.EscapeString(trimmed)
		model.Content = sanitized
		existing.Content = sanitized
	}

	if dto.Status != nil {
		valid := false
		for _, s := range ValidStatuses {
			if *dto.Status == s {
				valid = true
				break
			}
		}
		if !valid {
			return fiber.NewError(400, fmt.Sprintf("invalid status value (allowed: %v)", ValidStatuses))
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

func (h *CommentHooks) DeleteHook(c *fiber.Ctx, id any) error {
	ctx := auth.Context(c)

	existing, err := h.getComment(ctx, id)
	if err != nil {
		return fiber.NewError(404, "Comment not found")
	}

	user := auth.GetAuthenticatedUser(c)
	if user != nil && existing.UserId != nil && *existing.UserId != user.UserID {
		if !h.isModerator(c) {
			return fiber.NewError(403, "You can only delete your own comments")
		}
	}

	return nil
}

func (h *CommentHooks) GetByIDHook(c *fiber.Ctx, id any) error {
	return nil
}

func (h *CommentHooks) GetAllHook(c *fiber.Ctx, conditions *[]query.Condition, orderBy *[]crud.OrderByClause) error {
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

func (h *CommentHooks) getComment(ctx context.Context, id any) (*Comment, error) {
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
