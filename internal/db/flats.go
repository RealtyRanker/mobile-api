package db

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"github.com/asmisnik/mobile-api/internal/model"
	"github.com/asmisnik/mobile-api/internal/scoring"
	"github.com/asmisnik/mobile-api/internal/sqlite"
)

const flatColumns = `
	id, link, parsed_at, region, deal_type, price, flat_score,
	underground_score, underground_place, underground_distance_info, underground_stations,
	room_number, total_area, living_area, kitchen_area, floor, max_floor,
	deposit, deposit_months, comission, renovation, is_apartments,
	loggia_count, balcony_count, windows_view,
	separated_bathroom_count, combined_bathroom_count,
	has_dishwasher, has_conditioner, children_allowed, pets_allowed,
	ceiling_height, building_entrances_number, building_apartments_number, building_elevators_number,
	latitude, longitude,
	sale_type, mortgage_allowed, is_new_building, new_building_name, is_by_homeowner, demolished_in_moscow_program,
	photo_urls, photo_thumbnail_urls`

func scanFlat(row sqlite.Row, extra ...any) (model.Flat, error) {
	var f model.Flat
	fields := []any{
		&f.ID, &f.Link, sqlite.Time(&f.ParsedAt), &f.Region, &f.DealType, &f.Price, &f.Score,
		&f.UndergroundScore, &f.UndergroundPlace, &f.UndergroundDistanceInfo, sqlite.JSON(&f.UndergroundStations),
		&f.RoomNumber, &f.TotalArea, &f.LivingArea, &f.KitchenArea, &f.Floor, &f.MaxFloor,
		&f.Deposit, &f.DepositMonths, &f.Comission, &f.Renovation, &f.IsApartments,
		&f.LoggiaCount, &f.BalconyCount, &f.WindowsView,
		&f.SeparatedBathroomCount, &f.CombinedBathroomCount,
		&f.HasDishwasher, &f.HasConditioner, &f.ChildrenAllowed, &f.PetsAllowed,
		&f.CeilingHeight, &f.BuildingEntrancesNumber, &f.BuildingApartmentsNumber, &f.BuildingElevatorsNumber,
		&f.Latitude, &f.Longitude,
		&f.SaleType, &f.MortgageAllowed, &f.IsNewBuilding, &f.NewBuildingName, &f.IsByHomeowner, &f.DemolishedInMoscowProgram,
		sqlite.JSON(&f.PhotoURLs), sqlite.JSON(&f.PhotoThumbnailURLs),
	}
	err := row.Scan(append(fields, extra...)...)
	return f, err
}

func (db *DB) GetFlat(ctx context.Context, id int) (*model.Flat, error) {
	row := db.pool.QueryRowContext(ctx, `SELECT `+flatColumns+` FROM flats_history WHERE id = ?1`, id)
	f, err := scanFlat(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("loading flat: %w", err)
	}
	return &f, nil
}

// candidateWindow bounds how many recently-parsed matching rows are pulled
// into memory to recompute a custom score; the feed for a custom-scoring
// subscription ranks within this window rather than the whole table.
const candidateWindow = 500

const defaultPageSize = 20
const maxPageSize = 50

// Feed returns one page of flats_history rows matching sub's filters,
// best-first. cursor is an opaque string returned as nextCursor on the
// previous page ("" for the first page); the returned nextCursor is empty
// once there are no more results.
func (db *DB) Feed(ctx context.Context, sub model.Subscription, cursor string, limit int) (flats []model.Flat, nextCursor string, err error) {
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	if sub.Scoring != nil {
		return db.feedWithCustomScoring(ctx, sub, cursor, limit)
	}
	return db.feedDefaultScoring(ctx, sub, cursor, limit)
}

func (db *DB) feedDefaultScoring(ctx context.Context, sub model.Subscription, cursor string, limit int) ([]model.Flat, string, error) {
	where, args := filterClause(sub, sub.MinScore)

	if cursor != "" {
		cScore, cID, err := decodeScoreCursor(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
		where += fmt.Sprintf(" AND (flat_score, id) < (?%d, ?%d)", len(args)+1, len(args)+2)
		args = append(args, cScore, cID)
	}

	query := fmt.Sprintf(
		`SELECT %s FROM flats_history WHERE %s ORDER BY flat_score DESC, id DESC LIMIT %d`,
		flatColumns, where, limit+1,
	)
	rows, err := db.pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("querying feed: %w", err)
	}
	defer rows.Close()

	var flats []model.Flat
	for rows.Next() {
		f, err := scanFlat(rows)
		if err != nil {
			return nil, "", fmt.Errorf("scanning flat: %w", err)
		}
		flats = append(flats, f)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var next string
	if len(flats) > limit {
		last := flats[limit-1]
		next = encodeScoreCursor(last.Score, last.ID)
		flats = flats[:limit]
	}
	return flats, next, nil
}

func (db *DB) feedWithCustomScoring(ctx context.Context, sub model.Subscription, cursor string, limit int) ([]model.Flat, string, error) {
	offset := 0
	if cursor != "" {
		var err error
		offset, err = decodeOffsetCursor(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
	}

	// min_score is applied after recomputing the custom score below, not in
	// SQL, since flat_score in the table is the default-weighted score.
	where, args := filterClause(sub, 0)
	query := fmt.Sprintf(
		`SELECT %s FROM flats_history WHERE %s ORDER BY parsed_at DESC LIMIT %d`,
		flatColumns, where, candidateWindow,
	)
	rows, err := db.pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("querying feed candidates: %w", err)
	}
	defer rows.Close()

	custom := customParamsFromModel(sub.Scoring)
	var candidates []model.Flat
	for rows.Next() {
		f, err := scanFlat(rows)
		if err != nil {
			return nil, "", fmt.Errorf("scanning flat: %w", err)
		}
		f.Score = scoring.Score(toScoringFlat(f), custom)
		if sub.MinScore > 0 && int64(f.Score) < sub.MinScore {
			continue
		}
		candidates = append(candidates, f)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].ID > candidates[j].ID
	})

	if offset >= len(candidates) {
		return nil, "", nil
	}
	end := offset + limit
	var next string
	if end < len(candidates) {
		next = encodeOffsetCursor(end)
	} else {
		end = len(candidates)
	}
	return candidates[offset:end], next, nil
}

// toScoringFlat is the identity conversion kept as a named step so a future
// divergence between model.Flat and the scoring package's expected shape
// only needs a change here.
func toScoringFlat(f model.Flat) *model.Flat {
	return &f
}

func customParamsFromModel(p *model.ScoringParams) *scoring.CustomParams {
	if p == nil {
		return nil
	}
	return &scoring.CustomParams{
		AllArea:            p.AllArea,
		KitchenArea:        p.KitchenArea,
		Pets:               p.Pets,
		Dishwasher:         p.Dishwasher,
		Conditioner:        p.Conditioner,
		Apartments:         p.Apartments,
		TwoRoom:            p.TwoRoom,
		ThreeRoom:          p.ThreeRoom,
		FourRoom:           p.FourRoom,
		AdditionalRooms:    p.AdditionalRooms,
		WindowsYard:        p.WindowsYard,
		WindowsStreet:      p.WindowsStreet,
		WindowsBoth:        p.WindowsBoth,
		RenovationDesign:   p.RenovationDesign,
		RenovationEuro:     p.RenovationEuro,
		RenovationCosmetic: p.RenovationCosmetic,
		BathroomSeparated:  p.BathroomSeparated,
		Balcony:            p.Balcony,
		Loggia:             p.Loggia,
		Underground:        p.Underground,
	}
}
