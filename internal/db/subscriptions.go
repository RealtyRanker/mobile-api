package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/asmisnik/mobile-api/internal/model"
	"github.com/asmisnik/mobile-api/internal/sqlite"
)

const subscriptionColumns = `
	id, name, is_active, created_at, deal_type, region,
	min_price, max_price, min_area, max_area, rooms, min_score,
	metro_stations, metro_filter_label,
	min_underground_place, min_kitchen_area, min_floor, max_floor, min_ceiling_height,
	children_required, pets_required, dishwasher_required, conditioner_required,
	min_renovation, balcony_required, bathroom_type, last_viewed_at`

func scanSubscription(row sqlite.Row) (model.Subscription, error) {
	var s model.Subscription
	var lastViewedAt any // scanned but not exposed to the client directly
	err := row.Scan(
		&s.ID, &s.Name, &s.IsActive, sqlite.Time(&s.CreatedAt), &s.DealType, &s.Region,
		&s.MinPrice, &s.MaxPrice, &s.MinArea, &s.MaxArea, sqlite.JSON(&s.Rooms), &s.MinScore,
		sqlite.JSON(&s.MetroStations), &s.MetroFilterLabel,
		&s.MinUndergroundPlace, &s.MinKitchenArea, &s.MinFloor, &s.MaxFloor, &s.MinCeilingHeight,
		&s.ChildrenRequired, &s.PetsRequired, &s.DishwasherRequired, &s.ConditionerRequired,
		&s.MinRenovation, &s.BalconyRequired, &s.BathroomType, &lastViewedAt,
	)
	return s, err
}

// CreateSubscription inserts a new saved search (and its custom scoring
// params, if any) for userID.
func (db *DB) CreateSubscription(ctx context.Context, userID int, s model.Subscription) (int, error) {
	rooms := s.Rooms
	if rooms == nil {
		rooms = []int64{}
	}
	metroStations := s.MetroStations
	if metroStations == nil {
		metroStations = []string{}
	}

	tx, err := db.pool.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var id int
	err = tx.QueryRowContext(ctx,
		`INSERT INTO app_subscriptions
		   (user_id, name, deal_type, region, min_price, max_price, min_area, max_area, rooms, min_score,
		    metro_stations, metro_filter_label,
		    min_underground_place, min_kitchen_area, min_floor, max_floor, min_ceiling_height,
		    children_required, pets_required, dishwasher_required, conditioner_required,
		    min_renovation, balcony_required, bathroom_type)
		 VALUES (?1,?2,?3,?4,?5,?6,?7,?8,?9,?10,?11,?12,?13,?14,?15,?16,?17,?18,?19,?20,?21,?22,?23,?24)
		 RETURNING id`,
		userID, s.Name, s.DealType, s.Region, s.MinPrice, s.MaxPrice, s.MinArea, s.MaxArea, sqlite.Ints(rooms), s.MinScore,
		sqlite.Strings(metroStations), s.MetroFilterLabel,
		s.MinUndergroundPlace, s.MinKitchenArea, s.MinFloor, s.MaxFloor, s.MinCeilingHeight,
		s.ChildrenRequired, s.PetsRequired, s.DishwasherRequired, s.ConditionerRequired,
		s.MinRenovation, s.BalconyRequired, s.BathroomType,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("inserting subscription: %w", err)
	}

	if err := upsertScoringParamsTx(ctx, tx, id, s.Scoring); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return id, nil
}

// UpdateSubscription overwrites an existing saved search owned by userID.
func (db *DB) UpdateSubscription(ctx context.Context, userID, id int, s model.Subscription) error {
	rooms := s.Rooms
	if rooms == nil {
		rooms = []int64{}
	}
	metroStations := s.MetroStations
	if metroStations == nil {
		metroStations = []string{}
	}

	tx, err := db.pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	tag, err := tx.ExecContext(ctx,
		`UPDATE app_subscriptions SET
		   name = ?1, deal_type = ?2, region = ?3, min_price = ?4, max_price = ?5,
		   min_area = ?6, max_area = ?7, rooms = ?8, min_score = ?9,
		   metro_stations = ?10, metro_filter_label = ?11,
		   min_underground_place = ?12, min_kitchen_area = ?13, min_floor = ?14, max_floor = ?15,
		   min_ceiling_height = ?16, children_required = ?17, pets_required = ?18,
		   dishwasher_required = ?19, conditioner_required = ?20,
		   min_renovation = ?21, balcony_required = ?22, bathroom_type = ?23
		 WHERE id = ?24 AND user_id = ?25`,
		s.Name, s.DealType, s.Region, s.MinPrice, s.MaxPrice,
		s.MinArea, s.MaxArea, sqlite.Ints(rooms), s.MinScore,
		sqlite.Strings(metroStations), s.MetroFilterLabel,
		s.MinUndergroundPlace, s.MinKitchenArea, s.MinFloor, s.MaxFloor,
		s.MinCeilingHeight, s.ChildrenRequired, s.PetsRequired,
		s.DishwasherRequired, s.ConditionerRequired,
		s.MinRenovation, s.BalconyRequired, s.BathroomType,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("updating subscription: %w", err)
	}
	if changed, err := tag.RowsAffected(); err != nil {
		return err
	} else if changed == 0 {
		return ErrNotFound
	}

	if err := upsertScoringParamsTx(ctx, tx, id, s.Scoring); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func upsertScoringParamsTx(ctx context.Context, tx *sql.Tx, subscriptionID int, p *model.ScoringParams) error {
	if p == nil {
		_, err := tx.ExecContext(ctx, `DELETE FROM app_subscription_scoring_params WHERE subscription_id = ?1`, subscriptionID)
		if err != nil {
			return fmt.Errorf("clearing scoring params: %w", err)
		}
		return nil
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO app_subscription_scoring_params
		   (subscription_id, all_area_multiplier, kitchen_area_multiplier, pets_multiplier,
		    dishwasher_multiplier, conditioner_multiplier, apartments_multiplier,
		    two_room_multiplier, three_room_multiplier, four_room_multiplier, additional_rooms_multiplier,
		    windows_yard_multiplier, windows_street_multiplier, windows_both_multiplier,
		    renovation_design_mult, renovation_euro_mult, renovation_cosmetic_mult,
		    bathroom_separated_mult, balcony_multiplier, loggia_multiplier, underground_score_mult)
		 VALUES (?1,?2,?3,?4,?5,?6,?7,?8,?9,?10,?11,?12,?13,?14,?15,?16,?17,?18,?19,?20,?21)
		 ON CONFLICT (subscription_id) DO UPDATE SET
		   all_area_multiplier = EXCLUDED.all_area_multiplier,
		   kitchen_area_multiplier = EXCLUDED.kitchen_area_multiplier,
		   pets_multiplier = EXCLUDED.pets_multiplier,
		   dishwasher_multiplier = EXCLUDED.dishwasher_multiplier,
		   conditioner_multiplier = EXCLUDED.conditioner_multiplier,
		   apartments_multiplier = EXCLUDED.apartments_multiplier,
		   two_room_multiplier = EXCLUDED.two_room_multiplier,
		   three_room_multiplier = EXCLUDED.three_room_multiplier,
		   four_room_multiplier = EXCLUDED.four_room_multiplier,
		   additional_rooms_multiplier = EXCLUDED.additional_rooms_multiplier,
		   windows_yard_multiplier = EXCLUDED.windows_yard_multiplier,
		   windows_street_multiplier = EXCLUDED.windows_street_multiplier,
		   windows_both_multiplier = EXCLUDED.windows_both_multiplier,
		   renovation_design_mult = EXCLUDED.renovation_design_mult,
		   renovation_euro_mult = EXCLUDED.renovation_euro_mult,
		   renovation_cosmetic_mult = EXCLUDED.renovation_cosmetic_mult,
		   bathroom_separated_mult = EXCLUDED.bathroom_separated_mult,
		   balcony_multiplier = EXCLUDED.balcony_multiplier,
		   loggia_multiplier = EXCLUDED.loggia_multiplier,
		   underground_score_mult = EXCLUDED.underground_score_mult`,
		subscriptionID, p.AllArea, p.KitchenArea, p.Pets, p.Dishwasher, p.Conditioner, p.Apartments,
		p.TwoRoom, p.ThreeRoom, p.FourRoom, p.AdditionalRooms,
		p.WindowsYard, p.WindowsStreet, p.WindowsBoth,
		p.RenovationDesign, p.RenovationEuro, p.RenovationCosmetic,
		p.BathroomSeparated, p.Balcony, p.Loggia, p.Underground,
	)
	if err != nil {
		return fmt.Errorf("upserting scoring params: %w", err)
	}
	return nil
}

func scanScoringParams(row sqlite.Row) (*model.ScoringParams, error) {
	var p model.ScoringParams
	err := row.Scan(
		&p.AllArea, &p.KitchenArea, &p.Pets, &p.Dishwasher, &p.Conditioner, &p.Apartments,
		&p.TwoRoom, &p.ThreeRoom, &p.FourRoom, &p.AdditionalRooms,
		&p.WindowsYard, &p.WindowsStreet, &p.WindowsBoth,
		&p.RenovationDesign, &p.RenovationEuro, &p.RenovationCosmetic,
		&p.BathroomSeparated, &p.Balcony, &p.Loggia, &p.Underground,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (db *DB) loadScoringParams(ctx context.Context, subscriptionID int) (*model.ScoringParams, error) {
	row := db.pool.QueryRowContext(ctx,
		`SELECT all_area_multiplier, kitchen_area_multiplier, pets_multiplier,
		        dishwasher_multiplier, conditioner_multiplier, apartments_multiplier,
		        two_room_multiplier, three_room_multiplier, four_room_multiplier, additional_rooms_multiplier,
		        windows_yard_multiplier, windows_street_multiplier, windows_both_multiplier,
		        renovation_design_mult, renovation_euro_mult, renovation_cosmetic_mult,
		        bathroom_separated_mult, balcony_multiplier, loggia_multiplier, underground_score_mult
		 FROM app_subscription_scoring_params WHERE subscription_id = ?1`,
		subscriptionID,
	)
	p, err := scanScoringParams(row)
	if err != nil {
		return nil, fmt.Errorf("loading scoring params: %w", err)
	}
	return p, nil
}

// ListSubscriptions returns every saved search for userID, newest first,
// each annotated with NewCount: the number of matching listings parsed
// since the search was last opened (see TouchLastViewed). For a
// custom-scoring search the count is computed against the default
// flat_score column as an approximation — good enough for a "N new" badge,
// exact ranking happens when the feed itself is fetched.
func (db *DB) ListSubscriptions(ctx context.Context, userID int) ([]model.Subscription, error) {
	rows, err := db.pool.QueryContext(ctx,
		`SELECT `+subscriptionColumns+` FROM app_subscriptions WHERE user_id = ?1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []model.Subscription
	for rows.Next() {
		s, err := scanSubscription(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning subscription: %w", err)
		}
		subs = append(subs, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range subs {
		params, err := db.loadScoringParams(ctx, subs[i].ID)
		if err != nil {
			return nil, err
		}
		subs[i].Scoring = params

		count, err := db.countNewMatches(ctx, subs[i])
		if err != nil {
			return nil, err
		}
		subs[i].NewCount = count
	}
	return subs, nil
}

func (db *DB) countNewMatches(ctx context.Context, s model.Subscription) (int, error) {
	where, args := filterClause(s, s.MinScore)
	query := fmt.Sprintf(
		`SELECT count(*) FROM flats_history
		 WHERE %s AND parsed_at > (SELECT last_viewed_at FROM app_subscriptions WHERE id = ?%d)`,
		where, len(args)+1,
	)
	args = append(args, s.ID)

	var count int
	if err := db.pool.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting new matches: %w", err)
	}
	return count, nil
}

// GetSubscription loads one saved search owned by userID.
func (db *DB) GetSubscription(ctx context.Context, userID, id int) (*model.Subscription, error) {
	row := db.pool.QueryRowContext(ctx,
		`SELECT `+subscriptionColumns+` FROM app_subscriptions WHERE id = ?1 AND user_id = ?2`,
		id, userID,
	)
	s, err := scanSubscription(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("loading subscription: %w", err)
	}
	params, err := db.loadScoringParams(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	s.Scoring = params
	return &s, nil
}

func (db *DB) DeleteSubscription(ctx context.Context, userID, id int) error {
	tag, err := db.pool.ExecContext(ctx, `DELETE FROM app_subscriptions WHERE id = ?1 AND user_id = ?2`, id, userID)
	if err != nil {
		return fmt.Errorf("deleting subscription: %w", err)
	}
	if changed, err := tag.RowsAffected(); err != nil {
		return err
	} else if changed == 0 {
		return ErrNotFound
	}
	return nil
}

func (db *DB) SetSubscriptionActive(ctx context.Context, userID, id int, active bool) error {
	tag, err := db.pool.ExecContext(ctx,
		`UPDATE app_subscriptions SET is_active = ?1 WHERE id = ?2 AND user_id = ?3`,
		active, id, userID,
	)
	if err != nil {
		return fmt.Errorf("updating subscription active state: %w", err)
	}
	if changed, err := tag.RowsAffected(); err != nil {
		return err
	} else if changed == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchLastViewed resets the "new since" watermark used by NewCount to now.
func (db *DB) TouchLastViewed(ctx context.Context, userID, id int) error {
	tag, err := db.pool.ExecContext(ctx,
		`UPDATE app_subscriptions SET last_viewed_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER) WHERE id = ?1 AND user_id = ?2`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("touching last_viewed_at: %w", err)
	}
	if changed, err := tag.RowsAffected(); err != nil {
		return err
	} else if changed == 0 {
		return ErrNotFound
	}
	return nil
}
