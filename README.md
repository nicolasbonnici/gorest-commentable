# GoREST Commentable Plugin

[![Test](https://github.com/nicolasbonnici/gorest-commentable/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/nicolasbonnici/gorest-commentable/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nicolasbonnici/gorest-commentable)](https://goreportcard.com/report/github.com/nicolasbonnici/gorest-commentable)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A polymorphic commenting plugin for GoREST that allows adding comments to any resource type.

## Features

- **Polymorphic Comments**: Add comments to any resource type
- **Nested Comments**: Support for hierarchical comment threads
- **Configurable Allowed Types**: Control which resource types can be commented on
- **Content Validation**: XSS protection and content length limits
- **User Association**: Optional user authentication integration
- **Pagination**: Built-in pagination support for comment lists
- **Go Migrations**: Database schema managed via Go code (not SQL files)

## Installation

```bash
go get github.com/nicolasbonnici/gorest-commentable
```


## Development Environment

To set up your development environment:

```bash
make install
```

This will:
- Install Go dependencies
- Install development tools (golangci-lint)
- Set up git hooks (pre-commit linting and tests)

## Configuration

```yaml
plugins:
  - name: commentable
    enabled: true
    config:
      allowed_types: ["post", "article", "product"]
      max_content_length: 10000
      pagination_limit: 20
      max_pagination_limit: 100
      enable_nesting: true
      max_nesting_depth: 10
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `allowed_types` | `[]string` | `["post"]` | Resource types that can receive comments |
| `max_content_length` | `int` | `10000` | Maximum comment content length in bytes |
| `pagination_limit` | `int` | `20` | Default pagination limit |
| `max_pagination_limit` | `int` | `100` | Maximum allowed pagination limit |
| `enable_nesting` | `bool` | `true` | Allow nested/threaded comments |
| `max_nesting_depth` | `int` | `10` | Maximum nesting depth for replies |

## API Endpoints

### List Comments
```
GET /comments?commentable=post&commentableId={id}
```

### Get Comment
```
GET /comments/:id
```

### Create Comment
```
POST /comments
Content-Type: application/json

{
  "commentableId": "uuid",
  "commentable": "post",
  "parentId": "uuid",  // optional, for nested comments
  "content": "Comment text"
}
```

### Update Comment
```
PUT /comments/:id
Content-Type: application/json

{
  "content": "Updated comment text"
}
```

### Delete Comment
```
DELETE /comments/:id
```

## Advanced Filtering

### Array Filters (Multiple Values)

Get comments for multiple resource types using either syntax:

**Without brackets (recommended for simplicity):**
```bash
GET /comments?commentable=post&commentable=article
```

**With brackets (explicit array syntax):**
```bash
GET /comments?commentable[]=post&commentable[]=article
```

Both generate SQL: `WHERE commentable IN ('post', 'article')`

### Exclusion Filters (NOT IN)

Exclude specific types (requires GoREST v0.5.0+):

```bash
GET /comments?commentable[nin]=draft&commentable[nin]=deleted
```

Generates SQL: `WHERE commentable NOT IN ('draft', 'deleted')`

### Combining Filters

Filters are combined with AND logic:

```bash
GET /comments?commentable=post&user_id=123e4567-e89b-12d3-a456-426614174000
```

Generates SQL: `WHERE commentable IN ('post') AND user_id = '123e4567...'`

### Available Filter Fields

- `commentable` - Resource type (validates against configured allowed_types)
- `commentable_id` - Resource UUID
- `user_id` - Comment author UUID
- `parent_id` - Parent comment UUID (null for top-level comments)
- `created_at[gte]` - Created on or after date
- `created_at[lte]` - Created on or before date
- `updated_at[gte]` - Updated on or after date
- `updated_at[lte]` - Updated on or before date

### Filter Operators

- `field=value` - Equality (single value)
- `field=val1&field=val2` - IN operator (multiple values)
- `field[]=val1&field[]=val2` - IN operator (explicit array syntax)
- `field[nin]=val1&field[nin]=val2` - NOT IN operator
- `field[gt]=value` - Greater than
- `field[gte]=value` - Greater than or equal
- `field[lt]=value` - Less than
- `field[lte]=value` - Less than or equal
- `field[like]=pattern` - Pattern match (case-sensitive)
- `field[ilike]=pattern` - Pattern match (case-insensitive)

### Limits

- Maximum 50 values per filter field
- Invalid commentable types return 400 error with allowed types list

### Examples

**Get comments on posts and articles:**
```bash
GET /comments?commentable=post&commentable=article
```

**Get recent comments (last 7 days):**
```bash
GET /comments?created_at[gte]=2024-01-20T00:00:00Z
```

**Get comments excluding drafts:**
```bash
GET /comments?commentable[nin]=draft
```

**Combine filters with pagination and ordering:**
```bash
GET /comments?commentable=post&limit=20&order[created_at]=desc
```

## Database Schema

```sql
CREATE TABLE comment (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    commentable_id UUID NOT NULL,
    commentable TEXT NOT NULL,
    parent_id UUID REFERENCES comment(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    updated_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_commentable ON comment(commentable, commentable_id, created_at);
CREATE INDEX idx_user_id ON comment(user_id);
CREATE INDEX idx_parent_id ON comment(parent_id);
```

## Usage Example

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/nicolasbonnici/gorest"
    "github.com/nicolasbonnici/gorest-commentable"
)

func main() {
    app := fiber.New()

    // Initialize plugin with configuration
    plugin := commentable.NewPlugin()

    config := map[string]interface{}{
        "database": db,
        "allowed_types": []interface{}{"post", "article"},
        "max_content_length": 5000,
        "pagination_limit": 20,
    }

    if err := plugin.Initialize(config); err != nil {
        panic(err)
    }

    plugin.SetupEndpoints(app)

    app.Listen(":3000")
}
```

## Development

### Run Tests
```bash
make test
```

### Run Linter
```bash
make lint
```

### Build
```bash
make build
```

### Coverage Report
```bash
make coverage
```

## Security

- **XSS Protection**: All comment content is HTML-escaped
- **Content Length Limits**: Prevents extremely large payloads
- **Type Validation**: Only configured resource types are allowed
- **Foreign Key Constraints**: Maintains referential integrity where possible

---

## Git Hooks

This directory contains git hooks for the GoREST plugin to maintain code quality.

### Available Hooks

#### pre-commit

Runs before each commit to ensure code quality:
- **Linting**: Runs `make lint` to check code style and potential issues
- **Tests**: Runs `make test` to verify all tests pass

### Installation

#### Automatic Installation

Run the install script from the project root:

```bash
./.githooks/install.sh
```

#### Manual Installation

Copy the hooks to your `.git/hooks` directory:

```bash
cp .githooks/pre-commit .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

---


## License

See [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please ensure:
- All tests pass
- Code is linted
- New features have test coverage
- Documentation is updated

## Part of GoREST Ecosystem

- [GoREST](https://github.com/nicolasbonnici/gorest) - Core framework
- [GoREST Auth](https://github.com/nicolasbonnici/gorest-auth) - Authentication plugin
- [GoREST Likeable](https://github.com/nicolasbonnici/gorest-likeable) - Like/reaction plugin
- [GoREST Blog](https://github.com/nicolasbonnici/gorest-blog) - Blog plugin
