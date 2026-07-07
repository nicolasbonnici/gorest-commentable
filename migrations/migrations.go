package migrations

import (
	"context"

	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/migrations"
)

func GetMigrations() migrations.MigrationSource {
	builder := migrations.NewMigrationBuilder("gorest-commentable")

	builder.Add(
		"20260102000001000",
		"create_comments_table",
		func(ctx context.Context, db database.Database) error {
			if err := migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `CREATE TABLE IF NOT EXISTS comment (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					user_id UUID REFERENCES users(id) ON DELETE SET NULL,
					commentable_id UUID NOT NULL,
					commentable TEXT NOT NULL,
					parent_id UUID REFERENCES comment(id) ON DELETE CASCADE,
					content TEXT NOT NULL,
					updated_at TIMESTAMP(0) WITH TIME ZONE,
					created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`,
				MySQL: `CREATE TABLE IF NOT EXISTS comment (
					id CHAR(36) PRIMARY KEY,
					user_id CHAR(36),
					commentable_id CHAR(36) NOT NULL,
					commentable VARCHAR(255) NOT NULL,
					parent_id CHAR(36),
					content TEXT NOT NULL,
					updated_at TIMESTAMP NULL,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
					FOREIGN KEY (parent_id) REFERENCES comment(id) ON DELETE CASCADE,
					INDEX idx_commentable (commentable, commentable_id, created_at),
					INDEX idx_user_id (user_id),
					INDEX idx_parent_id (parent_id)
				) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
				SQLite: `CREATE TABLE IF NOT EXISTS comment (
					id TEXT PRIMARY KEY,
					user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
					commentable_id TEXT NOT NULL,
					commentable TEXT NOT NULL,
					parent_id TEXT REFERENCES comment(id) ON DELETE CASCADE,
					content TEXT NOT NULL,
					updated_at DATETIME,
					created_at DATETIME NOT NULL DEFAULT (datetime('now'))
				)`,
			}); err != nil {
				return err
			}

			// Create indexes for Postgres and SQLite
			if db.DriverName() == "postgres" {
				// Composite index for commentable queries
				if err := migrations.SQL(ctx, db, migrations.DialectSQL{
					Postgres: `CREATE INDEX IF NOT EXISTS idx_commentable ON comment(commentable, commentable_id, created_at)`,
				}); err != nil {
					return err
				}
				if err := migrations.CreateIndex(ctx, db, "idx_user_id", "comment", "user_id"); err != nil {
					return err
				}
				if err := migrations.CreateIndex(ctx, db, "idx_parent_id", "comment", "parent_id"); err != nil {
					return err
				}
			}

			if db.DriverName() == "sqlite" {
				// Composite index for commentable queries
				if err := migrations.SQL(ctx, db, migrations.DialectSQL{
					SQLite: `CREATE INDEX IF NOT EXISTS idx_commentable ON comment(commentable, commentable_id, created_at)`,
				}); err != nil {
					return err
				}
				if err := migrations.CreateIndex(ctx, db, "idx_user_id", "comment", "user_id"); err != nil {
					return err
				}
				if err := migrations.CreateIndex(ctx, db, "idx_parent_id", "comment", "parent_id"); err != nil {
					return err
				}
			}

			return nil
		},
		func(ctx context.Context, db database.Database) error {
			// Drop indexes first
			if db.DriverName() == "postgres" || db.DriverName() == "sqlite" {
				_ = migrations.DropIndex(ctx, db, "idx_commentable", "comment")
				_ = migrations.DropIndex(ctx, db, "idx_user_id", "comment")
				_ = migrations.DropIndex(ctx, db, "idx_parent_id", "comment")
			}

			return migrations.DropTableIfExists(ctx, db, "comment")
		},
	)

	builder.Add(
		"20260210000001000",
		"add_status_to_comments",
		func(ctx context.Context, db database.Database) error {
			return migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `ALTER TABLE comment ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'awaiting' CHECK (status IN ('awaiting', 'published', 'moderated', 'draft'))`,
				MySQL:    `ALTER TABLE comment ADD COLUMN status ENUM('awaiting', 'published', 'moderated', 'draft') NOT NULL DEFAULT 'awaiting'`,
				SQLite:   `ALTER TABLE comment ADD COLUMN status TEXT NOT NULL DEFAULT 'awaiting' CHECK (status IN ('awaiting', 'published', 'moderated', 'draft'))`,
			})
		},
		func(ctx context.Context, db database.Database) error {
			return migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `ALTER TABLE comment DROP COLUMN status`,
				MySQL:    `ALTER TABLE comment DROP COLUMN status`,
				SQLite:   `ALTER TABLE comment DROP COLUMN status`,
			})
		},
	)

	builder.Add(
		"20260211000001000",
		"add_ip_and_ua_to_comments",
		func(ctx context.Context, db database.Database) error {
			// SQLite only permits a single column per ALTER TABLE, so each
			// column is added with its own statement to stay portable.
			if err := migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `ALTER TABLE comment ADD COLUMN ip_address VARCHAR(45)`,
				MySQL:    `ALTER TABLE comment ADD COLUMN ip_address VARCHAR(45)`,
				SQLite:   `ALTER TABLE comment ADD COLUMN ip_address TEXT`,
			}); err != nil {
				return err
			}
			return migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `ALTER TABLE comment ADD COLUMN user_agent TEXT`,
				MySQL:    `ALTER TABLE comment ADD COLUMN user_agent TEXT`,
				SQLite:   `ALTER TABLE comment ADD COLUMN user_agent TEXT`,
			})
		},
		func(ctx context.Context, db database.Database) error {
			if err := migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `ALTER TABLE comment DROP COLUMN ip_address`,
				MySQL:    `ALTER TABLE comment DROP COLUMN ip_address`,
				SQLite:   `ALTER TABLE comment DROP COLUMN ip_address`,
			}); err != nil {
				return err
			}
			return migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `ALTER TABLE comment DROP COLUMN user_agent`,
				MySQL:    `ALTER TABLE comment DROP COLUMN user_agent`,
				SQLite:   `ALTER TABLE comment DROP COLUMN user_agent`,
			})
		},
	)

	builder.Add(
		"20260508000001000",
		"add_remote_source_tracking_to_comments",
		func(ctx context.Context, db database.Database) error {
			// SQLite only permits a single column per ALTER TABLE.
			if err := migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `ALTER TABLE comment ADD COLUMN remote_source_id TEXT`,
				MySQL:    `ALTER TABLE comment ADD COLUMN remote_source_id TEXT`,
				SQLite:   `ALTER TABLE comment ADD COLUMN remote_source_id TEXT`,
			}); err != nil {
				return err
			}
			if err := migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `ALTER TABLE comment ADD COLUMN remote_source TEXT`,
				MySQL:    `ALTER TABLE comment ADD COLUMN remote_source VARCHAR(255)`,
				SQLite:   `ALTER TABLE comment ADD COLUMN remote_source TEXT`,
			}); err != nil {
				return err
			}

			// Create unique index to prevent duplicate imports
			return migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `CREATE UNIQUE INDEX idx_comment_remote_source ON comment(remote_source_id, remote_source) WHERE remote_source_id IS NOT NULL AND remote_source IS NOT NULL`,
				MySQL:    `CREATE UNIQUE INDEX idx_comment_remote_source ON comment(remote_source_id, remote_source)`,
				SQLite:   `CREATE UNIQUE INDEX idx_comment_remote_source ON comment(remote_source_id, remote_source) WHERE remote_source_id IS NOT NULL AND remote_source IS NOT NULL`,
			})
		},
		func(ctx context.Context, db database.Database) error {
			// Drop index first
			_ = migrations.DropIndex(ctx, db, "idx_comment_remote_source", "comment")

			if err := migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `ALTER TABLE comment DROP COLUMN remote_source_id`,
				MySQL:    `ALTER TABLE comment DROP COLUMN remote_source_id`,
				SQLite:   `ALTER TABLE comment DROP COLUMN remote_source_id`,
			}); err != nil {
				return err
			}
			return migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `ALTER TABLE comment DROP COLUMN remote_source`,
				MySQL:    `ALTER TABLE comment DROP COLUMN remote_source`,
				SQLite:   `ALTER TABLE comment DROP COLUMN remote_source`,
			})
		},
	)

	builder.Add(
		"20260706000001000",
		"add_thread_traversal_indexes",
		func(ctx context.Context, db database.Database) error {
			// The batch-by-depth thread fetch loads each nesting level with a
			// single "parent_id IN (...) AND status = ..." query; a composite
			// index on that exact pair keeps every level lookup index-only.
			// idx_commentable already leads with (commentable, commentable_id),
			// so the polymorphic pair lookup for root comments is covered.
			if err := migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `CREATE INDEX IF NOT EXISTS idx_comment_parent_status ON comment(parent_id, status)`,
				MySQL:    `CREATE INDEX idx_comment_parent_status ON comment(parent_id, status)`,
				SQLite:   `CREATE INDEX IF NOT EXISTS idx_comment_parent_status ON comment(parent_id, status)`,
			}); err != nil {
				return err
			}

			return migrations.SQL(ctx, db, migrations.DialectSQL{
				Postgres: `CREATE INDEX IF NOT EXISTS idx_comment_commentable_pair ON comment(commentable, commentable_id)`,
				MySQL:    `CREATE INDEX idx_comment_commentable_pair ON comment(commentable, commentable_id)`,
				SQLite:   `CREATE INDEX IF NOT EXISTS idx_comment_commentable_pair ON comment(commentable, commentable_id)`,
			})
		},
		func(ctx context.Context, db database.Database) error {
			_ = migrations.DropIndex(ctx, db, "idx_comment_parent_status", "comment")
			_ = migrations.DropIndex(ctx, db, "idx_comment_commentable_pair", "comment")
			return nil
		},
	)

	return builder.Build()
}
