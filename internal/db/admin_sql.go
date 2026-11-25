package db

import (
	"database/sql/driver"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"modernc.org/sqlite"
)

func init() {
	err := sqlite.RegisterDeterministicScalarFunction("rr_platform", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		link, _ := args[0].(string)
		return Platform(link), nil
	})
	if err != nil {
		panic(err)
	}
	err = sqlite.RegisterDeterministicScalarFunction("rr_updated", 2, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		raw, _ := args[0].(string)
		seen, _ := args[1].(int64)
		updated := listingUpdated(raw, time.UnixMicro(seen))
		if updated.IsZero() {
			return nil, nil
		}
		return updated.UnixMicro(), nil
	})
	if err != nil {
		panic(err)
	}
}

func Platform(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return "other"
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "cian.ru" || strings.HasSuffix(host, ".cian.ru"):
		return "cian"
	case host == "realty.yandex.ru" || host == "realty.ya.ru":
		return "yandex"
	case host == "domclick.ru" || strings.HasSuffix(host, ".domclick.ru"):
		return "domclick"
	default:
		return "other"
	}
}

var moscow = time.FixedZone("MSK", 3*60*60)
var relativeUpdate = regexp.MustCompile(`^([0-9]{1,5})\s+(секунд\p{L}*|минут\p{L}*|час\p{L}*|день|дня|дней|сут\p{L}*)\s+назад$`)
var dayUpdate = regexp.MustCompile(`^(сегодня|вчера)(?:[, ]+в?\s*([0-9]{1,2}:[0-9]{2}))?$`)

// Relative labels from the Cian search are anchored to the time of parsing.
func listingUpdated(raw string, seen time.Time) time.Time {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if stamp, err := time.Parse(time.RFC3339Nano, strings.ToUpper(raw)); err == nil {
		return stamp.UTC()
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02t15:04:05", "2006-01-02", "02.01.2006 15:04", "02.01.2006"} {
		if stamp, err := time.ParseInLocation(layout, raw, moscow); err == nil {
			return stamp.UTC()
		}
	}
	if seen.Unix() <= 0 {
		return time.Time{}
	}
	if match := relativeUpdate.FindStringSubmatch(raw); match != nil {
		n, _ := strconv.Atoi(match[1])
		unit := 24 * time.Hour
		switch {
		case strings.HasPrefix(match[2], "секунд"):
			unit = time.Second
		case strings.HasPrefix(match[2], "минут"):
			unit = time.Minute
		case strings.HasPrefix(match[2], "час"):
			unit = time.Hour
		}
		return seen.Add(-time.Duration(n) * unit).UTC()
	}
	if match := dayUpdate.FindStringSubmatch(raw); match != nil {
		day := seen.In(moscow)
		if match[1] == "вчера" {
			day = day.AddDate(0, 0, -1)
		}
		if match[2] == "" {
			return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, moscow).UTC()
		}
		if clock, err := time.Parse("15:04", match[2]); err == nil {
			return time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, moscow).UTC()
		}
	}
	return time.Time{}
}
