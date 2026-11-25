package db

import (
	"testing"

	"github.com/asmisnik/mobile-api/internal/model"
)

func TestFilterClauseBasic(t *testing.T) {
	sub := model.Subscription{
		DealType: "rent",
		Region:   1,
		MinPrice: 50000,
		MaxPrice: 150000,
		Rooms:    []int64{1, 2},
	}
	where, args := filterClause(sub, 100)

	want := "region = ?1 AND deal_type = ?2 AND price >= ?3 AND price <= ?4 AND room_number IN (SELECT value FROM json_each(?5)) AND flat_score >= ?6"
	if where != want {
		t.Fatalf("where clause mismatch:\n got: %s\nwant: %s", where, want)
	}
	wantArgs := []any{1, "rent", int64(50000), int64(150000), []int64{1, 2}, int64(100)}
	if len(args) != len(wantArgs) {
		t.Fatalf("expected %d args, got %d: %v", len(wantArgs), len(args), args)
	}
}

func TestFilterClauseSkipsZeroFilters(t *testing.T) {
	sub := model.Subscription{DealType: "sale", Region: 2}
	where, args := filterClause(sub, 0)
	want := "region = ?1 AND deal_type = ?2"
	if where != want {
		t.Fatalf("where clause mismatch:\n got: %s\nwant: %s", where, want)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d: %v", len(args), args)
	}
}

func TestRenovationAtLeast(t *testing.T) {
	cases := map[string][]string{
		"":         nil,
		"cosmetic": {"cosmetic", "euro", "design"},
		"euro":     {"euro", "design"},
		"design":   {"design"},
	}
	for level, want := range cases {
		got := renovationAtLeast(level)
		if len(got) != len(want) {
			t.Fatalf("renovationAtLeast(%q) = %v, want %v", level, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("renovationAtLeast(%q) = %v, want %v", level, got, want)
			}
		}
	}
}

func TestCursorRoundTrip(t *testing.T) {
	c := encodeScoreCursor(84213, 591)
	score, id, err := decodeScoreCursor(c)
	if err != nil || score != 84213 || id != 591 {
		t.Fatalf("score cursor round trip failed: score=%d id=%d err=%v", score, id, err)
	}

	oc := encodeOffsetCursor(40)
	offset, err := decodeOffsetCursor(oc)
	if err != nil || offset != 40 {
		t.Fatalf("offset cursor round trip failed: offset=%d err=%v", offset, err)
	}
}
