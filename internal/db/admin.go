package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/asmisnik/mobile-api/internal/model"
	"github.com/asmisnik/mobile-api/internal/sqlite"
)

type AdminDB struct {
	pool       *sql.DB
	hasAliases bool
	hasAddress bool
}

// OpenAdmin opens the existing catalog without write access.
func OpenAdmin(ctx context.Context, path string) (*AdminDB, error) {
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro&_pragma=busy_timeout(5000)&_pragma=query_only(ON)"}).String()
	pool, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(4)
	catalog := &AdminDB{pool: pool}
	var hasFlats bool
	err = pool.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM sqlite_schema WHERE type = 'table' AND name = 'flats_history'),
		EXISTS(SELECT 1 FROM sqlite_schema WHERE type = 'table' AND name = 'flat_listing_aliases'),
		EXISTS(SELECT 1 FROM pragma_table_info('flats_history') WHERE name = 'address')`,
	).Scan(&hasFlats, &catalog.hasAliases, &catalog.hasAddress)
	if err == nil && !hasFlats {
		err = fmt.Errorf("database does not contain flats_history")
	}
	if err != nil {
		pool.Close()
		return nil, err
	}
	return catalog, nil
}

func (db *AdminDB) Close() error { return db.pool.Close() }

type AdminFilter struct {
	model.Subscription
	Source    string
	Sort      string
	Page      int
	RoomsMore bool
}

type AdminCard struct {
	ID           int       `json:"id"`
	Link         string    `json:"link"`
	Source       string    `json:"source"`
	OtherSources []string  `json:"other_sources"`
	DealType     string    `json:"deal_type"`
	Region       int       `json:"region"`
	Price        int       `json:"price"`
	Area         float64   `json:"area"`
	Rooms        int       `json:"rooms"`
	Floor        int       `json:"floor"`
	MaxFloor     int       `json:"max_floor"`
	Score        int       `json:"score"`
	Address      string    `json:"address"`
	Photo        string    `json:"photo"`
	Stations     []string  `json:"stations"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AdminPage struct {
	Flats    []AdminCard `json:"flats"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

const adminPageSize = 24

func (db *AdminDB) List(ctx context.Context, filter AdminFilter) (AdminPage, error) {
	page := AdminPage{Flats: []AdminCard{}, PageSize: adminPageSize}
	where, args := listingFilterClause(filter.Subscription, filter.MinScore, true, filter.RoomsMore)
	if where == "" {
		where = "TRUE"
	}
	if filter.Source != "" {
		source := fmt.Sprintf("?%d", len(args)+1)
		match := "rr_platform(link) = " + source
		if db.hasAliases {
			match += " OR EXISTS (SELECT 1 FROM flat_listing_aliases a WHERE a.flat_id = flats_history.id AND rr_platform(a.link) = " + source + ")"
		}
		where += " AND (" + match + ")"
		args = append(args, filter.Source)
	}

	tx, err := db.pool.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return page, err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM flats_history WHERE "+where, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	lastPage := max(1, (page.Total+adminPageSize-1)/adminPageSize)
	page.Page = min(max(1, filter.Page), lastPage)
	if page.Total == 0 {
		return page, nil
	}
	order := "updated DESC, id DESC"
	switch filter.Sort {
	case "oldest":
		order = "updated ASC, id ASC"
	case "price_asc":
		order = "(price <= 0), price ASC, id DESC"
	case "price_desc":
		order = "price DESC, id DESC"
	case "score_desc":
		order = "flat_score DESC, id DESC"
	case "score_asc":
		order = "flat_score ASC, id DESC"
	}
	aliasSources, aliasPhoto := "'[]'", "NULL"
	if db.hasAliases {
		aliasSources = `(SELECT json_group_array(DISTINCT rr_platform(a.link)) FROM flat_listing_aliases a
			WHERE a.flat_id = flats_history.id AND rr_platform(a.link) <> rr_platform(flats_history.link))`
		aliasPhoto = `(SELECT COALESCE(NULLIF(json_extract(a.payload, '$.PhotoThumbnailURLs[0]'), ''),
			NULLIF(json_extract(a.payload, '$.PhotoURLs[0]'), '')) AS photo
			FROM flat_listing_aliases a WHERE a.flat_id = flats_history.id
			AND json_valid(a.payload) AND photo IS NOT NULL ORDER BY a.link LIMIT 1)`
	}
	query := `SELECT id, link, rr_platform(link), deal_type, region, price, total_area, room_number,
		floor, max_floor, flat_score, underground_stations, ` + db.addressColumn() + `,
		COALESCE(NULLIF(json_extract(photo_thumbnail_urls, '$[0]'), ''), NULLIF(json_extract(photo_urls, '$[0]'), ''), ` + aliasPhoto + `, ''),
		` + aliasSources + `,
		COALESCE(rr_updated(last_updated, parsed_at), parsed_at, 0) AS updated
		FROM flats_history WHERE ` + where + " ORDER BY " + order + " LIMIT ? OFFSET ?"
	rows, err := tx.QueryContext(ctx, query, append(args, adminPageSize, (page.Page-1)*adminPageSize)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var f AdminCard
		err := rows.Scan(&f.ID, &f.Link, &f.Source, &f.DealType, &f.Region, &f.Price, &f.Area,
			&f.Rooms, &f.Floor, &f.MaxFloor, &f.Score, sqlite.JSON(&f.Stations), &f.Address, &f.Photo,
			sqlite.JSON(&f.OtherSources), sqlite.Time(&f.UpdatedAt))
		if err != nil {
			return page, err
		}
		page.Flats = append(page.Flats, f)
	}
	return page, rows.Err()
}

func (db *AdminDB) addressColumn() string {
	if db.hasAddress {
		return "address"
	}
	return "''"
}

type AdminListing struct {
	Source        string   `json:"source"`
	Link          string   `json:"link"`
	Price         int      `json:"price"`
	PhotoURLs     []string `json:"photo_urls"`
	ThumbnailURLs []string `json:"photo_thumbnail_urls"`
	LastUpdated   string   `json:"last_updated"`
}

type AdminDetail struct {
	model.Flat
	Address   string         `json:"address"`
	UpdatedAt time.Time      `json:"updated_at"`
	Listings  []AdminListing `json:"listings"`
}

func (db *AdminDB) Detail(ctx context.Context, id int) (AdminDetail, error) {
	var result AdminDetail
	var lastUpdated string
	columns := strings.Replace(flatColumns, "id, link, parsed_at,", "id, link, COALESCE(parsed_at, 0),", 1)
	row := db.pool.QueryRowContext(ctx, "SELECT "+columns+", "+db.addressColumn()+", last_updated FROM flats_history WHERE id = ?", id)
	flat, err := scanFlat(row, &result.Address, &lastUpdated)
	if err == sql.ErrNoRows {
		return result, ErrNotFound
	}
	if err != nil {
		return result, err
	}
	result.Flat = flat
	result.UpdatedAt = flat.ParsedAt
	if date := listingUpdated(lastUpdated, flat.ParsedAt); !date.IsZero() {
		result.UpdatedAt = date
	}
	result.Listings = []AdminListing{{
		Source: Platform(flat.Link), Link: flat.Link, Price: flat.Price,
		PhotoURLs: flat.PhotoURLs, ThumbnailURLs: flat.PhotoThumbnailURLs, LastUpdated: lastUpdated,
	}}
	if !db.hasAliases {
		return result, nil
	}

	rows, err := db.pool.QueryContext(ctx, "SELECT link, payload FROM flat_listing_aliases WHERE flat_id = ? ORDER BY link", id)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var link, raw string
		if err := rows.Scan(&link, &raw); err != nil {
			return result, err
		}
		var payload struct {
			Address            string
			Price              int
			LastUpdated        string
			PhotoURLs          []string
			PhotoThumbnailURLs []string
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return result, fmt.Errorf("decoding listing alias: %w", err)
		}
		if result.Address == "" {
			result.Address = payload.Address
		}
		result.Listings = append(result.Listings, AdminListing{
			Source: Platform(link), Link: link, Price: payload.Price, LastUpdated: payload.LastUpdated,
			PhotoURLs: payload.PhotoURLs, ThumbnailURLs: payload.PhotoThumbnailURLs,
		})
	}
	return result, rows.Err()
}
