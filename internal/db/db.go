// Package db is mobile-api's SQLite access layer. It talks to the same
// realty_parser database the Telegram-facing services use, but through its
// own tables (app_users, app_refresh_tokens, app_subscriptions,
// app_subscription_scoring_params) so the app's account system never
// collides with Telegram chat ids, while still reading flats_history (owned
// by realty-parser/flats-analyzer) to build a feed.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/asmisnik/mobile-api/internal/model"
	"github.com/asmisnik/mobile-api/internal/sqlite"
)

type DB struct {
	pool *sql.DB
}

func New(ctx context.Context, dsn string) (*DB, error) {
	pool, err := sqlite.Open(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("opening SQLite database: %w", err)
	}
	return &DB{pool: pool}, nil
}

func (db *DB) Close() {
	db.pool.Close()
}

// ---- users ----

// UpsertDeviceUser finds or creates the app_users row for a device id (a
// random identifier the app generates once and persists locally — see
// deviceSignInRequest for why there's no external identity provider here).
// A blank displayName never overwrites a previously stored one.
func (db *DB) UpsertDeviceUser(ctx context.Context, deviceID, displayName string) (int, error) {
	var id int
	err := db.pool.QueryRowContext(ctx,
		`INSERT INTO app_users (device_id, display_name)
		 VALUES (?1, ?2)
		 ON CONFLICT (device_id) DO UPDATE SET
		   display_name = CASE WHEN EXCLUDED.display_name <> '' THEN EXCLUDED.display_name ELSE app_users.display_name END
		 RETURNING id`,
		deviceID, displayName,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upserting device user: %w", err)
	}
	return id, nil
}

func (db *DB) GetUser(ctx context.Context, userID int) (*model.User, error) {
	var u model.User
	err := db.pool.QueryRowContext(ctx,
		`SELECT id, email, display_name, created_at FROM app_users WHERE id = ?1`,
		userID,
	).Scan(&u.ID, &u.Email, &u.DisplayName, sqlite.Time(&u.CreatedAt))
	if err != nil {
		return nil, fmt.Errorf("loading user: %w", err)
	}
	return &u, nil
}

// DeleteUser removes the account and everything derived from it (refresh
// tokens, saved searches, scoring params) via ON DELETE CASCADE.
func (db *DB) DeleteUser(ctx context.Context, userID int) error {
	_, err := db.pool.ExecContext(ctx, `DELETE FROM app_users WHERE id = ?1`, userID)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	return nil
}

// ---- refresh tokens ----

func (db *DB) CreateRefreshToken(ctx context.Context, userID int, tokenHash string, expiresAt time.Time) error {
	_, err := db.pool.ExecContext(ctx,
		`INSERT INTO app_refresh_tokens (user_id, token_hash, expires_at) VALUES (?1, ?2, ?3)`,
		userID, tokenHash, expiresAt.UnixMicro(),
	)
	if err != nil {
		return fmt.Errorf("storing refresh token: %w", err)
	}
	return nil
}

// UserIDForRefreshToken returns the owning user id for a live (unrevoked,
// unexpired) refresh token hash.
func (db *DB) UserIDForRefreshToken(ctx context.Context, tokenHash string) (int, error) {
	var userID int
	err := db.pool.QueryRowContext(ctx,
		`SELECT user_id FROM app_refresh_tokens
		 WHERE token_hash = ?1 AND revoked_at IS NULL AND expires_at > CAST(unixepoch('subsec') * 1000000 AS INTEGER)`,
		tokenHash,
	).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("looking up refresh token: %w", err)
	}
	return userID, nil
}

func (db *DB) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := db.pool.ExecContext(ctx,
		`UPDATE app_refresh_tokens SET revoked_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER) WHERE token_hash = ?1 AND revoked_at IS NULL`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("revoking refresh token: %w", err)
	}
	return nil
}

// ErrNotFound is returned when a lookup scoped to a resource/owner pair
// matches no row (used to turn "not found" and "not yours" into the same
// 404 at the HTTP layer, instead of leaking which one it was).
var ErrNotFound = fmt.Errorf("not found")
