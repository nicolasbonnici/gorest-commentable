package commentable

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	_ "github.com/nicolasbonnici/gorest/database/sqlite"
	"github.com/nicolasbonnici/gorest/query"

	"github.com/nicolasbonnici/gorest-commentable/migrations"
)

// countingDB wraps a Database to count Query/QueryRow calls so tests can assert
// the batch-by-depth fetch issues one query per level rather than one per node.
type countingDB struct {
	database.Database
	queries int
}

func (c *countingDB) Query(ctx context.Context, q string, args ...interface{}) (database.Rows, error) {
	c.queries++
	return c.Database.Query(ctx, q, args...)
}

func (c *countingDB) QueryRow(ctx context.Context, q string, args ...interface{}) database.Row {
	c.queries++
	return c.Database.QueryRow(ctx, q, args...)
}

func setupThreadDB(t *testing.T) *countingDB {
	t.Helper()

	db, err := database.Open("sqlite", "file:"+t.TempDir()+"/thread.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if _, err := db.Exec(ctx, `CREATE TABLE users (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create users: %v", err)
	}

	source := migrations.GetMigrations()
	list, err := source.Migrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	for _, m := range list {
		if err := m.ExecuteUp(ctx, db); err != nil {
			t.Fatalf("migrate %s: %v", m.FullName(), err)
		}
	}

	return &countingDB{Database: db}
}

// insertComment persists one comment and returns its id, chaining parents to
// build a nesting tree of arbitrary depth.
func insertComment(t *testing.T, db database.Database, parentID *string, status string) string {
	t.Helper()
	id := uuid.New().String()
	c := crud.New[Comment](db)
	err := c.Create(context.Background(), Comment{
		Id:            id,
		CommentableId: "post-1",
		Commentable:   "post",
		ParentId:      parentID,
		Content:       "c",
		Status:        status,
	})
	if err != nil {
		t.Fatalf("insert comment: %v", err)
	}
	return id
}

func TestFetchThread_BatchesPerDepthNotPerNode(t *testing.T) {
	db := setupThreadDB(t)
	cfg := DefaultConfig()

	// Build a 10-level chain plus fan-out at each level so a per-node walk
	// would issue far more queries than a per-level batch.
	var chain []string
	var parent *string
	for depth := 0; depth < cfg.MaxNestingDepth; depth++ {
		id := insertComment(t, db, parent, StatusPublished)
		insertComment(t, db, parent, StatusPublished) // sibling widening each level
		chain = append(chain, id)
		p := id
		parent = &p
	}

	total := len(chain) * 2

	c := crud.New[Comment](db)
	db.queries = 0
	roots, err := fetchThread(context.Background(), c, &cfg, "post", "post-1",
		[]query.Condition{query.Eq("status", StatusPublished)})
	if err != nil {
		t.Fatalf("fetchThread: %v", err)
	}

	if db.queries > cfg.MaxNestingDepth {
		t.Errorf("expected at most %d queries (one per level), got %d for %d nodes",
			cfg.MaxNestingDepth, db.queries, total)
	}

	if got := countNodes(roots); got != total {
		t.Errorf("expected %d comments in tree, got %d", total, got)
	}
}

func TestFetchThread_RespectsMaxDepth(t *testing.T) {
	db := setupThreadDB(t)
	cfg := DefaultConfig()
	cfg.MaxNestingDepth = 3

	var parent *string
	for depth := 0; depth < 6; depth++ {
		id := insertComment(t, db, parent, StatusPublished)
		p := id
		parent = &p
	}

	c := crud.New[Comment](db)
	db.queries = 0
	roots, err := fetchThread(context.Background(), c, &cfg, "post", "post-1", nil)
	if err != nil {
		t.Fatalf("fetchThread: %v", err)
	}

	if db.queries > cfg.MaxNestingDepth {
		t.Errorf("expected at most %d queries, got %d", cfg.MaxNestingDepth, db.queries)
	}
	if got := countNodes(roots); got != cfg.MaxNestingDepth {
		t.Errorf("expected %d levels fetched, got %d", cfg.MaxNestingDepth, got)
	}
}

func TestFetchThread_FiltersStatus(t *testing.T) {
	db := setupThreadDB(t)
	cfg := DefaultConfig()

	root := insertComment(t, db, nil, StatusPublished)
	insertComment(t, db, &root, StatusAwaiting)
	insertComment(t, db, &root, StatusPublished)

	c := crud.New[Comment](db)
	roots, err := fetchThread(context.Background(), c, &cfg, "post", "post-1",
		[]query.Condition{query.Eq("status", StatusPublished)})
	if err != nil {
		t.Fatalf("fetchThread: %v", err)
	}

	if got := countNodes(roots); got != 2 {
		t.Errorf("expected 2 published comments, got %d", got)
	}
}

func countNodes(nodes []*CommentThreadDTO) int {
	n := 0
	for _, node := range nodes {
		n += 1 + countNodes(node.Children)
	}
	return n
}
