package commentable

import (
	"github.com/gofiber/fiber/v2"
	"github.com/nicolasbonnici/gorest-commentable/migrations"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/plugin"
)

type CommentablePlugin struct {
	config Config
	db     database.Database
}

func NewPlugin() plugin.Plugin {
	return &CommentablePlugin{}
}

func (p *CommentablePlugin) Name() string {
	return "commentable"
}

func (p *CommentablePlugin) Initialize(config map[string]interface{}) error {
	p.config = DefaultConfig()

	if db, ok := config["database"].(database.Database); ok {
		p.db = db
		p.config.Database = db
	}

	if allowedTypes, ok := config["allowed_types"].([]interface{}); ok {
		types := make([]string, 0, len(allowedTypes))
		for _, t := range allowedTypes {
			if str, ok := t.(string); ok {
				types = append(types, str)
			}
		}
		if len(types) > 0 {
			p.config.AllowedTypes = types
		}
	}

	if maxContentLength, ok := config["max_content_length"].(int); ok {
		p.config.MaxContentLength = maxContentLength
	}

	if paginationLimit, ok := config["pagination_limit"].(int); ok {
		p.config.PaginationLimit = paginationLimit
	}

	if maxPaginationLimit, ok := config["max_pagination_limit"].(int); ok {
		p.config.MaxPaginationLimit = maxPaginationLimit
	}

	if enableNesting, ok := config["enable_nesting"].(bool); ok {
		p.config.EnableNesting = enableNesting
	}

	if maxNestingDepth, ok := config["max_nesting_depth"].(int); ok {
		p.config.MaxNestingDepth = maxNestingDepth
	}

	return p.config.Validate()
}

func (p *CommentablePlugin) Handler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Next()
	}
}

func (p *CommentablePlugin) SetupEndpoints(app *fiber.App) error {
	if p.db == nil {
		return nil
	}

	RegisterRoutes(app, p.db, &p.config)
	return nil
}

func (p *CommentablePlugin) MigrationSource() interface{} {
	return migrations.GetMigrations()
}

func (p *CommentablePlugin) MigrationDependencies() []string {
	return []string{"auth", "rbac"}
}

func (p *CommentablePlugin) Dependencies() []string {
	return []string{"auth", "rbac"}
}

func (p *CommentablePlugin) GetOpenAPIResources() []plugin.OpenAPIResource {
	return []plugin.OpenAPIResource{{
		Name:          "comment",
		PluralName:    "comments",
		BasePath:      "/comments",
		Tags:          []string{"Comments"},
		ResponseModel: Comment{},
		CreateModel:   CreateCommentRequest{},
		UpdateModel:   UpdateCommentRequest{},
		Description:   "Nested comment system",
	}}
}
