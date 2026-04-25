package commentable

import (
	"github.com/gofiber/fiber/v2"
	rbac "github.com/nicolasbonnici/gorest/rbac"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/processor"
)

const MaxFilterValuesPerField = 50

type CommentResource struct {
	processor processor.Processor[Comment, CommentCreateDTO, CommentUpdateDTO, CommentResponseDTO]
}

func RegisterCommentRoutes(router fiber.Router, db database.Database, config *Config) {
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

	commentCRUD := crud.New[Comment](db)
	hooks := NewCommentHooks(db, config, voter)
	converter := &CommentConverter{}

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

	proc := processor.New(processor.ProcessorConfig[Comment, CommentCreateDTO, CommentUpdateDTO, CommentResponseDTO]{
		DB:                 db,
		CRUD:               commentCRUD,
		Converter:          converter,
		PaginationLimit:    config.PaginationLimit,
		PaginationMaxLimit: config.MaxPaginationLimit,
		FieldMap:           fieldMapping,
		AllowedFields:      []string{"id", "userId", "commentableId", "commentable", "parentId", "content", "status", "ipAddress", "userAgent", "updatedAt", "createdAt"},
	}).
		WithCreateHook(hooks.Create).
		WithUpdateHook(hooks.Update).
		WithDeleteHook(hooks.Delete).
		WithGetByIDHook(hooks.GetByID).
		WithGetAllHook(hooks.GetAll)

	res := &CommentResource{
		processor: proc,
	}

	router.Get("/comments", res.GetAll)
	router.Get("/comments/:id", res.GetByID)
	router.Post("/comments", res.Create)
	router.Put("/comments/:id", res.Update)
	router.Delete("/comments/:id", res.Delete)
}

func (r *CommentResource) Create(c *fiber.Ctx) error {
	return r.processor.Create(c)
}

func (r *CommentResource) GetByID(c *fiber.Ctx) error {
	return r.processor.GetByID(c)
}

func (r *CommentResource) GetAll(c *fiber.Ctx) error {
	return r.processor.GetAll(c)
}

func (r *CommentResource) Update(c *fiber.Ctx) error {
	return r.processor.Update(c)
}

func (r *CommentResource) Delete(c *fiber.Ctx) error {
	return r.processor.Delete(c)
}
