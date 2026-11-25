package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asmisnik/mobile-api/internal/db"
)

func TestCatalogFiltersDetailsAndAnalytics(t *testing.T) {
	catalog := fixture(t, true)
	handler := Handler(catalog)
	options := url.Values{"deal_type": {"rent"}, "region": {"1"}, "location_type": {"district"},
		"location": {"Академический"}, "source": {"yandex"}, "min_kitchen": {"9"}, "dishwasher": {"1"},
		"rooms": {"2"}, "sort": {"price_asc"}}
	var page db.AdminPage
	get(t, handler, "/api/flats?"+options.Encode(), &page)
	if page.Total != 1 || page.Flats[0].ID != 1 || page.Flats[0].Photo != "https://images.example/yandex.jpg" ||
		len(page.Flats[0].OtherSources) != 1 || page.Flats[0].OtherSources[0] != "yandex" {
		t.Fatalf("filtered page did not include the matching alias and its image: %+v", page)
	}
	get(t, handler, "/api/flats?deal_type=rent", &page)
	if page.Total != 2 || page.Flats[0].ID != 1 || !page.Flats[0].UpdatedAt.After(page.Flats[1].UpdatedAt) {
		t.Fatalf("update sort did not use timestamps from the platforms: %+v", page)
	}
	get(t, handler, "/api/flats?deal_type=rent&sort=score_desc", &page)
	if page.Flats[0].ID != 2 {
		t.Fatalf("wrong score order: %+v", page)
	}

	var detail db.AdminDetail
	get(t, handler, "/api/flats/1", &detail)
	if len(detail.Listings) != 2 || detail.Address != "Москва, улица, 5" || detail.Listings[1].Price != 80000 ||
		detail.Listings[1].PhotoURLs[0] != "https://images.example/yandex.jpg" || detail.KitchenArea != 10 {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	var metrics db.AdminMetrics
	get(t, handler, "/api/analytics?deal_type=rent&days=7", &metrics)
	if metrics.Listings != 3 || metrics.UniqueFlats != 2 || metrics.SharedFlats != 1 || len(metrics.Platforms) != 2 || len(metrics.Volume) == 0 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
	cian, yandex := metrics.Platforms[0], metrics.Platforms[1]
	if cian.Source != "cian" || cian.AveragePrice != 90000 || cian.MedianPrice != 90000 ||
		yandex.Source != "yandex" || yandex.AveragePrice != 80000 || yandex.WithPhotos != 1 {
		t.Fatalf("statistics used the wrong platform values: %+v", metrics.Platforms)
	}
	get(t, handler, "/api/analytics?deal_type=sale", &metrics)
	if metrics.Listings != 1 || metrics.Platforms[0].AveragePrice != 15_000_000 {
		t.Fatalf("sales mixed with rentals: %+v", metrics)
	}
}

func TestAdminWorksWithEarlierDatabaseAndRejectsForeignHost(t *testing.T) {
	handler := Handler(fixture(t, false))
	var detail db.AdminDetail
	get(t, handler, "/api/flats/1", &detail)
	if len(detail.Listings) != 1 || detail.Address != "" {
		t.Fatalf("incorrect fallback for older schema: %+v", detail)
	}
	var page db.AdminPage
	get(t, handler, "/api/flats", &page)
	if page.Total != 3 {
		t.Fatalf("incorrect total for older schema: %+v", page)
	}
	req := httptest.NewRequest(http.MethodGet, "http://untrusted.test/api/flats", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("foreign host returned %d", response.Code)
	}
}

func get(t *testing.T, handler http.Handler, path string, dst any) {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("%s: %d %s", path, response.Code, response.Body)
	}
	if err := json.Unmarshal(response.Body.Bytes(), dst); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T, aliases bool) *db.AdminDB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "catalog.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	migrationDir := "../../../dev-tips/migrations/sqlite"
	if value := os.Getenv("REALTYRANKER_TEST_SCHEMA"); value != "" {
		migrationDir = filepath.Dir(value)
	}
	names := []string{"001_initial.sql"}
	if aliases {
		names = append(names, "002_cross_platform_listings.sql", "003_listing_alias_lookup.sql")
	}
	for _, name := range names {
		migration, err := os.ReadFile(filepath.Join(migrationDir, name))
		if os.IsNotExist(err) && os.Getenv("REALTYRANKER_TEST_SCHEMA") == "" {
			t.Skip("deploy schema unavailable")
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	_, err = conn.Exec(`INSERT INTO flats_history
		(id, link, deal_type, price, total_area, kitchen_area, room_number, flat_score,
		 underground_stations, has_dishwasher, last_updated) VALUES
		(1, 'https://www.cian.ru/rent/flat/1/', 'rent', 80000, 55, 10, 2, 500, '["Академическая"]', true, '1 минуту назад'),
		(2, 'https://www.cian.ru/rent/flat/2/', 'rent', 100000, 62, 12, 3, 600, '["Домодедовская"]', true, 'вчера'),
		(3, 'https://www.cian.ru/sale/flat/3/', 'sale', 15000000, 45, 9, 1, 800, '[]', false, '')`)
	if err != nil {
		t.Fatal(err)
	}
	if aliases {
		_, err := conn.Exec(`INSERT INTO flat_listing_aliases (link, flat_id, payload) VALUES
			('https://realty.yandex.ru/offer/10/', 1,
			'{"Address":"Москва, улица, 5","Price":80000,"TotalArea":55,"PhotoURLs":["https://images.example/yandex.jpg"]}')`)
		if err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := db.OpenAdmin(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { catalog.Close() })
	return catalog
}

func TestAdminServesEmbeddedInterface(t *testing.T) {
	handler := Handler(nil)
	for _, path := range []string{"/", "/app.js", "/styles.css"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil))
		if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) == "" {
			t.Fatalf("missing page asset %s: %d", path, response.Code)
		}
	}
}
