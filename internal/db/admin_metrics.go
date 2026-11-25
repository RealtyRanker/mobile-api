package db

import (
	"context"
	"database/sql"
	"time"
)

type PlatformMetrics struct {
	Source         string  `json:"source"`
	Listings       int     `json:"listings"`
	UniqueFlats    int     `json:"unique_flats"`
	SharedFlats    int     `json:"shared_flats"`
	AveragePrice   float64 `json:"average_price"`
	MedianPrice    float64 `json:"median_price"`
	AverageArea    float64 `json:"average_area"`
	AverageSqMeter float64 `json:"average_sq_meter"`
	AverageScore   float64 `json:"average_score"`
	WithPhotos     int     `json:"with_photos"`
}

type VolumePoint struct {
	Day      string `json:"day"`
	Source   string `json:"source"`
	Listings int    `json:"listings"`
}

type AdminMetrics struct {
	UniqueFlats int               `json:"unique_flats"`
	SharedFlats int               `json:"shared_flats"`
	Listings    int               `json:"listings"`
	Platforms   []PlatformMetrics `json:"platforms"`
	Volume      []VolumePoint     `json:"volume"`
}

func (db *AdminDB) Metrics(ctx context.Context, dealType string, region, days int, now time.Time) (AdminMetrics, error) {
	result := AdminMetrics{Platforms: []PlatformMetrics{}, Volume: []VolumePoint{}}
	args := []any{dealType, region}
	since := int64(0)
	if days > 0 {
		local := now.In(moscow)
		start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, moscow)
		since = start.AddDate(0, 0, 1-days).UnixMicro()
	}
	args = append(args, since)
	scope := `WITH scope AS (
		SELECT id, link, price, total_area, flat_score, parsed_at,
		       max(json_array_length(photo_urls), json_array_length(photo_thumbnail_urls)) > 0 AS has_photo
		FROM flats_history WHERE deal_type = ?1 AND (?2 = 0 OR region = ?2)
		  AND (?3 = 0 OR parsed_at >= ?3)
	), occurrences AS (
		SELECT id, rr_platform(link) AS source, price, total_area AS area, flat_score AS score, parsed_at, has_photo
		FROM scope`
	if db.hasAliases {
		scope += `
		UNION ALL
		SELECT s.id, rr_platform(a.link),
		       CAST(json_extract(a.payload, '$.Price') AS REAL),
		       CAST(json_extract(a.payload, '$.TotalArea') AS REAL),
		       CAST(json_extract(a.payload, '$.FlatScore') AS REAL), s.parsed_at,
		       max(coalesce(json_array_length(a.payload, '$.PhotoURLs'), 0),
		           coalesce(json_array_length(a.payload, '$.PhotoThumbnailURLs'), 0)) > 0
		FROM flat_listing_aliases a JOIN scope s ON s.id = a.flat_id
		WHERE json_valid(a.payload)`
	}
	scope += `), shared AS (
		SELECT id FROM occurrences GROUP BY id HAVING COUNT(DISTINCT source) > 1
	)`
	tx, err := db.pool.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, scope+` SELECT
		(SELECT COUNT(*) FROM scope), (SELECT COUNT(*) FROM shared), (SELECT COUNT(*) FROM occurrences)`, args...).
		Scan(&result.UniqueFlats, &result.SharedFlats, &result.Listings)
	if err != nil {
		return result, err
	}
	result.Platforms, err = platformMetrics(ctx, tx, scope, args)
	if err != nil {
		return result, err
	}
	result.Volume, err = listingVolume(ctx, tx, scope, args)
	return result, err
}

func platformMetrics(ctx context.Context, tx *sql.Tx, scope string, args []any) ([]PlatformMetrics, error) {
	rows, err := tx.QueryContext(ctx, scope+`,
		sorted_prices AS (
			SELECT source, price, row_number() OVER (PARTITION BY source ORDER BY price) AS n,
			       count(*) OVER (PARTITION BY source) AS total
			FROM occurrences WHERE price > 0
		), medians AS (
			SELECT source, avg(price) AS median FROM sorted_prices
			WHERE n IN ((total+1)/2, (total+2)/2) GROUP BY source
		)
		SELECT o.source, count(*), count(DISTINCT o.id),
		       count(DISTINCT CASE WHEN s.id IS NOT NULL THEN o.id END),
		       coalesce(avg(CASE WHEN price > 0 THEN price END), 0), coalesce(m.median, 0),
		       coalesce(avg(CASE WHEN area > 0 THEN area END), 0),
		       coalesce(avg(CASE WHEN area > 0 AND price > 0 THEN 1.0*price/area END), 0),
		       coalesce(avg(NULLIF(score, 0)), 0), coalesce(sum(has_photo), 0)
		FROM occurrences o LEFT JOIN shared s ON s.id = o.id LEFT JOIN medians m ON m.source = o.source
		GROUP BY o.source ORDER BY count(*) DESC, o.source ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PlatformMetrics{}
	for rows.Next() {
		var m PlatformMetrics
		if err := rows.Scan(&m.Source, &m.Listings, &m.UniqueFlats, &m.SharedFlats, &m.AveragePrice, &m.MedianPrice,
			&m.AverageArea, &m.AverageSqMeter, &m.AverageScore, &m.WithPhotos); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func listingVolume(ctx context.Context, tx *sql.Tx, scope string, args []any) ([]VolumePoint, error) {
	rows, err := tx.QueryContext(ctx, scope+`
		SELECT date(parsed_at/1000000, 'unixepoch', '+3 hours') AS day, source, count(*)
		FROM occurrences WHERE parsed_at > 0
		GROUP BY day, source ORDER BY day DESC, source`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []VolumePoint{}
	for rows.Next() {
		var point VolumePoint
		if err := rows.Scan(&point.Day, &point.Source, &point.Listings); err != nil {
			return nil, err
		}
		result = append(result, point)
	}
	return result, rows.Err()
}
