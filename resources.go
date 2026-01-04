package commentable

import (
	"net/url"

	"github.com/gofiber/fiber/v2"
	auth "github.com/nicolasbonnici/gorest-auth"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/filter"
	"github.com/nicolasbonnici/gorest/pagination"
	"github.com/nicolasbonnici/gorest/response"
)

type CommentResource struct {
	DB                 database.Database
	CRUD               *crud.CRUD[Comment]
	Config             *Config
	PaginationLimit    int
	PaginationMaxLimit int
}

func RegisterCommentRoutes(app *fiber.App, db database.Database, config *Config) {
	res := &CommentResource{
		DB:                 db,
		CRUD:               crud.New[Comment](db),
		Config:             config,
		PaginationLimit:    config.PaginationLimit,
		PaginationMaxLimit: config.MaxPaginationLimit,
	}

	app.Get("/comments", res.List)
	app.Get("/comments/:id", res.Get)
	app.Post("/comments", res.Create)
	app.Put("/comments/:id", res.Update)
	app.Delete("/comments/:id", res.Delete)
}

func (r *CommentResource) List(c *fiber.Ctx) error {
	limit := pagination.ParseIntQuery(c, "limit", r.PaginationLimit, r.PaginationMaxLimit)
	page := pagination.ParseIntQuery(c, "page", 1, 10000)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	includeCount := c.Query("count", "true") != "false"

	allowedFields := []string{"id", "user_id", "commentable_id", "commentable", "parent_id", "content", "updated_at", "created_at"}

	queryParams := make(url.Values)
	for key, value := range c.Context().QueryArgs().All() {
		queryParams.Add(string(key), string(value))
	}

	filters := filter.NewFilterSet(allowedFields, r.DB.Dialect())
	if err := filters.ParseFromQuery(queryParams); err != nil {
		return pagination.SendPaginatedError(c, 400, err.Error())
	}
	whereClause, whereArgs := filters.BuildWhereClause()

	ordering := filter.NewOrderSet(allowedFields)
	if err := ordering.ParseFromQuery(queryParams); err != nil {
		return pagination.SendPaginatedError(c, 400, err.Error())
	}
	orderByClause := ordering.BuildOrderByClause()

	result, err := r.CRUD.GetAllPaginated(auth.Context(c), crud.PaginationOptions{
		Limit:         limit,
		Offset:        offset,
		IncludeCount:  includeCount,
		WhereClause:   whereClause,
		WhereArgs:     whereArgs,
		OrderByClause: orderByClause,
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

	return response.SendFormatted(c, 200, item)
}

func (r *CommentResource) Create(c *fiber.Ctx) error {
	var req CreateCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validate request
	if err := req.Validate(r.Config); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	var item Comment
	item.CommentableId = req.CommentableId
	item.Commentable = req.Commentable
	item.ParentId = req.ParentId
	item.Content = req.Content

	if user := auth.GetAuthenticatedUser(c); user != nil {
		item.UserId = &user.UserID
	}

	ctx := auth.Context(c)
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

	// Validate request
	if err := req.Validate(r.Config); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// Get existing comment
	existing, err := r.CRUD.GetByID(auth.Context(c), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Not found"})
	}

	// Update only the content
	existing.Content = req.Content

	if err := r.CRUD.Update(auth.Context(c), id, *existing); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return response.SendFormatted(c, 200, existing)
}

func (r *CommentResource) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := r.CRUD.Delete(auth.Context(c), id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(204)
}
