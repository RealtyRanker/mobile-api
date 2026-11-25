package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/asmisnik/mobile-api/internal/model"
)

func TestMobileAuthSubscriptionsAndFeed(t *testing.T) {
	database, _ := testDB(t)
	ctx := context.Background()
	userID, err := database.UpsertDeviceUser(ctx, "device-1", "Имя")
	if err != nil {
		t.Fatal(err)
	}
	if id, err := database.UpsertDeviceUser(ctx, "device-1", ""); err != nil || id != userID {
		t.Fatalf("upsert: %d, %v", id, err)
	}
	user, err := database.GetUser(ctx, userID)
	if err != nil || user.DisplayName != "Имя" || user.CreatedAt.IsZero() {
		t.Fatalf("user: %+v, %v", user, err)
	}
	if err := database.CreateRefreshToken(ctx, userID, "live", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if id, err := database.UserIDForRefreshToken(ctx, "live"); err != nil || id != userID {
		t.Fatalf("token: %d, %v", id, err)
	}
	if err := database.CreateRefreshToken(ctx, userID, "expired", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.UserIDForRefreshToken(ctx, "expired"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired token: %v", err)
	}
	if err := database.RevokeRefreshToken(ctx, "live"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.UserIDForRefreshToken(ctx, "live"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked token: %v", err)
	}

	sub := model.Subscription{Name: "Тест", DealType: "rent", Region: 1, Rooms: []int64{2}, MetroStations: []string{"Орехово"}, MinRenovation: "euro", PetsRequired: true}
	id, err := database.CreateSubscription(ctx, userID, sub)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := database.GetSubscription(ctx, userID, id)
	if err != nil || len(loaded.Rooms) != 1 || loaded.MetroStations[0] != "Орехово" || loaded.CreatedAt.IsZero() {
		t.Fatalf("subscription: %+v, %v", loaded, err)
	}
	if _, err := database.GetSubscription(ctx, userID+1, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("owner scope: %v", err)
	}
	for i, station := range []string{"Орехово", "Орехово", "Другая"} {
		_, err := database.pool.ExecContext(ctx, `INSERT INTO flats_history
			(link, room_number, underground_stations, renovation, pets_allowed, flat_score, parsed_at, photo_urls)
			VALUES (?1, 2, json_array(?2), 'design', TRUE, ?3, ?4, '["https://example.test/photo.jpg"]')`,
			string(rune('a'+i)), station, 100+i, time.Now().Add(time.Second).UnixMicro())
		if err != nil {
			t.Fatal(err)
		}
	}
	page, cursor, err := database.Feed(ctx, *loaded, "", 1)
	if err != nil || len(page) != 1 || cursor == "" || page[0].Score != 101 || page[0].PhotoURLs[0] != "https://example.test/photo.jpg" || page[0].ParsedAt.IsZero() {
		t.Fatalf("feed first: %+v, %q, %v", page, cursor, err)
	}
	page, cursor, err = database.Feed(ctx, *loaded, cursor, 1)
	if err != nil || len(page) != 1 || cursor != "" || page[0].Score != 100 {
		t.Fatalf("feed second: %+v, %q, %v", page, cursor, err)
	}
	subs, err := database.ListSubscriptions(ctx, userID)
	if err != nil || len(subs) != 1 || subs[0].NewCount != 2 {
		t.Fatalf("new counts: %+v, %v", subs, err)
	}
	if _, err := database.pool.ExecContext(ctx, `UPDATE flats_history SET parsed_at = ?1`, time.Now().Add(-time.Minute).UnixMicro()); err != nil {
		t.Fatal(err)
	}
	if err := database.TouchLastViewed(ctx, userID, id); err != nil {
		t.Fatal(err)
	}
	subs, err = database.ListSubscriptions(ctx, userID)
	if err != nil || subs[0].NewCount != 0 {
		t.Fatalf("last viewed: %+v, %v", subs, err)
	}
	sub.Scoring = &model.ScoringParams{AllArea: 10.5}
	sub.MaxPrice = 1_000_000_000_000_000_000
	if err := database.UpdateSubscription(ctx, userID, id, sub); err != nil {
		t.Fatal(err)
	}
	loaded, err = database.GetSubscription(ctx, userID, id)
	if err != nil || loaded.Scoring == nil || loaded.Scoring.AllArea != 10.5 || loaded.MaxPrice != sub.MaxPrice {
		t.Fatalf("custom scoring: %+v, %v", loaded, err)
	}
	if page, _, err := database.Feed(ctx, *loaded, "", 10); err != nil || len(page) != 2 {
		t.Fatalf("custom feed: %+v, %v", page, err)
	}
	if err := database.SetSubscriptionActive(ctx, userID, id, false); err != nil {
		t.Fatal(err)
	}
	if err := database.DeleteUser(ctx, userID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := database.pool.QueryRowContext(ctx, `SELECT count(*) FROM app_subscription_scoring_params`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cascade: %d, %v", count, err)
	}
}
