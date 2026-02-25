package commentable

import (
	"fmt"
	"net/url"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	auth "github.com/nicolasbonnici/gorest-auth"
	rbac "github.com/nicolasbonnici/gorest-rbac"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/filter"
	"github.com/nicolasbonnici/gorest/pagination"
	"github.com/nicolasbonnici/gorest/response"
)

const MaxFilterValuesPerField = 50

type CommentResource struct {
	DB                 database.Database
	CRUD               *crud.CRUD[Comment]
	Config             *Config
	PaginationLimit    int
	PaginationMaxLimit int
	Voter              rbac.Voter
}

func RegisterCommentRoutes(app *fiber.App, db database.Database, config *Config) {
	rbacConfig := rbac.Config{
		DefaultPolicy: rbac.DenyAll,
		SuperuserRole: "admin",
		RoleHierarchy: map[string][]string{
			"writer":    {"moderator"},
			"moderator": {"reader"},
		},
		CacheEnabled:       true,
		CacheTTL:           300,
		StrictMode:         false,
		DefaultFieldPolicy: "deny",
	}

	voter, err := rbac.NewVoter(rbacConfig)
	if err != nil {
		panic("failed to create RBAC voter: " + err.Error())
	}

	res := &CommentResource{
		DB:                 db,
		CRUD:               crud.New[Comment](db),
		Config:             config,
		PaginationLimit:    config.PaginationLimit,
		PaginationMaxLimit: config.MaxPaginationLimit,
		Voter:              voter,
	}

	app.Get("/comments", res.List)
	app.Get("/comments/:id", res.Get)
	app.Post("/comments", res.Create)
	app.Put("/comments/:id", res.Update)
	app.Delete("/comments/:id", res.Delete)
}

func (r *CommentResource) isModerator(c *fiber.Ctx) bool {
	roles, ok := rbac.GetRoles(auth.Context(c))
	if !ok || len(roles) == 0 {
		return false
	}
	return r.Voter.IsSuperuser(roles) || rbac.HasRole(roles, "moderator", r.Voter.GetConfig().RoleHierarchy)
}

// validateCommentableFilter validates that commentable filter values are allowed types
func (r *CommentResource) validateCommentableFilter(filters *filter.FilterSet) error {
	for _, f := range filters.Filters {
		if f.Field == "commentable" {
			for _, val := range f.Values {
				if !r.Config.IsAllowedType(val) {
					return fmt.Errorf("invalid commentable type '%s' (allowed: %v)", val, r.Config.AllowedTypes)
				}
			}
		}
	}
	return nil
}

// validateFilterLimits prevents abuse by limiting array filter sizes
func (r *CommentResource) validateFilterLimits(filters *filter.FilterSet) error {
	for _, f := range filters.Filters {
		if len(f.Values) > MaxFilterValuesPerField {
			return fmt.Errorf("too many filter values for field '%s' (max: %d, got: %d)",
				f.Field, MaxFilterValuesPerField, len(f.Values))
		}
	}
	return nil
}

func (r *CommentResource) List(c *fiber.Ctx) error {
	limit := pagination.ParseIntQuery(c, "limit", r.PaginationLimit, r.PaginationMaxLimit)
	page := pagination.ParseIntQuery(c, "page", 1, 10000)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	includeCount := c.Query("count", "true") != "false"

	// Field mapping: JSON field name -> DB column name
	fieldMapping := map[string]string{
		"id":            "id",
		"userId":        "user_id",
		"commentableId": "commentable_id",
		"commentable":   "commentable",
		"parentId":      "parent_id",
		"content":       "content",
		"status":        "status",
		"ipAddress":     "ip_address",
		"userAgent":     "user_agent",
		"updatedAt":     "updated_at",
		"createdAt":     "created_at",
	}

	queryParams := make(url.Values)
	for key, value := range c.Context().QueryArgs().All() {
		queryParams.Add(string(key), string(value))
	}

	filters := filter.NewFilterSetWithMapping(fieldMapping, r.DB.Dialect())
	if err := filters.ParseFromQuery(queryParams); err != nil {
		return pagination.SendPaginatedError(c, 400, err.Error())
	}

	// Validate commentable types
	if err := r.validateCommentableFilter(filters); err != nil {
		return pagination.SendPaginatedError(c, 400, err.Error())
	}

	// Validate filter limits
	if err := r.validateFilterLimits(filters); err != nil {
		return pagination.SendPaginatedError(c, 400, err.Error())
	}

	if !r.isModerator(c) {
		filters.Filters = append(filters.Filters, filter.Filter{
			Field:    "status",
			Operator: filter.OpEqual,
			Values:   []string{StatusPublished},
		})
	}

	ordering := filter.NewOrderSetWithMapping(fieldMapping)
	if err := ordering.ParseFromQuery(queryParams); err != nil {
		return pagination.SendPaginatedError(c, 400, err.Error())
	}

	// Convert filter.OrderClause to crud.OrderByClause
	filterOrderClauses := ordering.OrderClauses()
	orderByClauses := make([]crud.OrderByClause, len(filterOrderClauses))
	for i, oc := range filterOrderClauses {
		orderByClauses[i] = crud.OrderByClause{
			Column:    oc.Column,
			Direction: oc.Direction,
		}
	}

	result, err := r.CRUD.GetAllPaginated(auth.Context(c), crud.PaginationOptions{
		Limit:        limit,
		Offset:       offset,
		IncludeCount: includeCount,
		Conditions:   filters.Conditions(),
		OrderBy:      orderByClauses,
	})
	if err != nil {
		return pagination.SendPaginatedError(c, 500, err.Error())
	}

	return pagination.SendHydraCollection(c, result.Items, result.Total, limit, page, r.PaginationLimit)
}

func (r *CommentResource) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	item, err := r.CRUD.GetByID(auth.Context(c), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Not found"})
	}

	if item.Status != StatusPublished && !r.isModerator(c) {
		return c.Status(404).JSON(fiber.Map{"error": "Not found"})
	}

	return response.SendFormatted(c, 200, item)
}

func (r *CommentResource) Create(c *fiber.Ctx) error {
	var req CreateCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := req.Validate(r.Config); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	ctx := auth.Context(c)

	var item Comment
	item.Id = uuid.New().String() // Generate UUID before insert
	item.CommentableId = req.CommentableId
	item.Commentable = req.Commentable
	item.ParentId = req.ParentId
	item.Content = req.Content
	item.Status = r.Config.DefaultStatus

	if user := auth.GetAuthenticatedUser(c); user != nil {
		item.UserId = &user.UserID
	} else {
		// For unauthenticated users, store IP and User Agent
		ipAddr := c.IP()
		userAgent := c.Get("User-Agent")
		item.IpAddress = &ipAddr
		item.UserAgent = &userAgent
	}

	if err := r.CRUD.Create(ctx, item); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	created, err := r.CRUD.GetByID(ctx, item.Id)
	if err != nil {
		return response.SendFormatted(c, 201, item)
	}

	return response.SendFormatted(c, 201, created)
}

func (r *CommentResource) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var req UpdateCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := req.Validate(r.Config); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	ctx := auth.Context(c)

	existing, err := r.CRUD.GetByID(ctx, id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Not found"})
	}

	if req.Content != nil {
		user := auth.GetAuthenticatedUser(c)
		if user == nil || existing.UserId == nil || *existing.UserId != user.UserID {
			return c.Status(403).JSON(fiber.Map{"error": "You can only edit your own comments"})
		}
		existing.Content = *req.Content
	}

	// Update status if provided (RBAC will handle moderator permission check)
	if req.Status != nil {
		updateItem := *existing
		updateItem.Status = *req.Status

		// Clear read-only fields before RBAC validation
		updateItem.Id = ""
		updateItem.IpAddress = nil
		updateItem.UserAgent = nil
		updateItem.CreatedAt = nil
		updateItem.UpdatedAt = nil

		if err := r.Voter.ValidateWrite(ctx, &updateItem); err != nil {
			return c.Status(403).JSON(fiber.Map{"error": err.Error()})
		}

		existing.Status = *req.Status
	}

	if err := r.CRUD.Update(ctx, id, *existing); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return response.SendFormatted(c, 200, existing)
}

func (r *CommentResource) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	ctx := auth.Context(c)

	existing, err := r.CRUD.GetByID(ctx, id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Not found"})
	}

	user := auth.GetAuthenticatedUser(c)
	if user == nil || existing.UserId == nil || *existing.UserId != user.UserID {
		return c.Status(403).JSON(fiber.Map{"error": "You can only delete your own comments"})
	}

	if err := r.CRUD.Delete(ctx, id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(204)
}
