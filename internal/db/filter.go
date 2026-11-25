package db

import (
	"fmt"
	"strings"

	"github.com/asmisnik/mobile-api/internal/model"
	"github.com/asmisnik/mobile-api/internal/sqlite"
)

// renovationAtLeast returns the renovation values that satisfy "at least
// level X", mirroring flats-analyzer's renovationRank ordering
// ("" < cosmetic < euro < design).
func renovationAtLeast(level string) []string {
	switch level {
	case "cosmetic":
		return []string{"cosmetic", "euro", "design"}
	case "euro":
		return []string{"euro", "design"}
	case "design":
		return []string{"design"}
	default:
		return nil
	}
}

// filterClause builds the WHERE conditions (and matching args, starting at
// ?1) that select flats_history rows matching a subscription's filters,
// mirroring flats-analyzer's matchesSubscription. minScore, when > 0, is
// applied against the flat_score column directly; pass 0 to skip it (the
// caller recomputes score in Go when the subscription has custom scoring
// weights).
func filterClause(sub model.Subscription, minScore int64) (string, []any) {
	return listingFilterClause(sub, minScore, false, false)
}

func listingFilterClause(sub model.Subscription, minScore int64, optionalScope, roomsMore bool) (string, []any) {
	var b strings.Builder
	var args []any

	next := func() int { return len(args) + 1 }

	if !optionalScope || sub.Region != 0 {
		fmt.Fprintf(&b, " AND region = ?%d", next())
		args = append(args, sub.Region)
	}
	if !optionalScope || sub.DealType != "" {
		fmt.Fprintf(&b, " AND deal_type = ?%d", next())
		args = append(args, sub.DealType)
	}

	if sub.MinPrice > 0 {
		fmt.Fprintf(&b, " AND price >= ?%d", next())
		args = append(args, sub.MinPrice)
	}
	if sub.MaxPrice > 0 {
		fmt.Fprintf(&b, " AND price <= ?%d", next())
		args = append(args, sub.MaxPrice)
	}
	if sub.MinArea > 0 {
		fmt.Fprintf(&b, " AND total_area >= ?%d", next())
		args = append(args, sub.MinArea)
	}
	if sub.MaxArea > 0 {
		fmt.Fprintf(&b, " AND total_area <= ?%d", next())
		args = append(args, sub.MaxArea)
	}
	if len(sub.Rooms) > 0 {
		if roomsMore {
			fmt.Fprintf(&b, " AND (room_number IN (SELECT value FROM json_each(?%d)) OR room_number >= 6)", next())
		} else {
			fmt.Fprintf(&b, " AND room_number IN (SELECT value FROM json_each(?%d))", next())
		}
		args = append(args, sqlite.Ints(sub.Rooms))
	} else if roomsMore {
		b.WriteString(" AND room_number >= 6")
	}
	if minScore > 0 {
		fmt.Fprintf(&b, " AND flat_score >= ?%d", next())
		args = append(args, minScore)
	}
	if sub.MinUndergroundPlace > 0 {
		fmt.Fprintf(&b, " AND underground_place > 0 AND underground_place <= ?%d", next())
		args = append(args, sub.MinUndergroundPlace)
	}
	if len(sub.MetroStations) > 0 {
		fmt.Fprintf(&b, " AND EXISTS (SELECT 1 FROM json_each(underground_stations) AS station JOIN json_each(?%d) AS wanted ON station.value = wanted.value)", next())
		args = append(args, sqlite.Strings(sub.MetroStations))
	}
	if sub.MinKitchenArea > 0 {
		fmt.Fprintf(&b, " AND kitchen_area >= ?%d", next())
		args = append(args, sub.MinKitchenArea)
	}
	if sub.MinFloor > 0 {
		fmt.Fprintf(&b, " AND floor >= ?%d", next())
		args = append(args, sub.MinFloor)
	}
	if sub.MaxFloor > 0 {
		fmt.Fprintf(&b, " AND floor <= ?%d", next())
		args = append(args, sub.MaxFloor)
	}
	if sub.MinCeilingHeight > 0 {
		fmt.Fprintf(&b, " AND ceiling_height >= ?%d", next())
		args = append(args, sub.MinCeilingHeight)
	}
	if sub.ChildrenRequired {
		b.WriteString(" AND children_allowed = TRUE")
	}
	if sub.PetsRequired {
		b.WriteString(" AND pets_allowed = TRUE")
	}
	if sub.DishwasherRequired {
		b.WriteString(" AND has_dishwasher = TRUE")
	}
	if sub.ConditionerRequired {
		b.WriteString(" AND has_conditioner = TRUE")
	}
	if levels := renovationAtLeast(sub.MinRenovation); levels != nil {
		fmt.Fprintf(&b, " AND renovation IN (SELECT value FROM json_each(?%d))", next())
		args = append(args, sqlite.Strings(levels))
	}
	if sub.BalconyRequired {
		b.WriteString(" AND (balcony_count > 0 OR loggia_count > 0)")
	}
	switch sub.BathroomType {
	case "separated":
		b.WriteString(" AND separated_bathroom_count > 0")
	case "combined":
		b.WriteString(" AND combined_bathroom_count > 0")
	}

	return strings.TrimPrefix(b.String(), " AND "), args
}
