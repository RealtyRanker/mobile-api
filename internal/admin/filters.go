package admin

import (
	"fmt"
	"math"
	"net/url"
	"strconv"

	"github.com/asmisnik/mobile-api/internal/db"
	"github.com/asmisnik/mobile-api/internal/metro"
	"github.com/asmisnik/mobile-api/internal/model"
	"github.com/asmisnik/mobile-api/internal/region"
)

type locationChoice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type filterOptions struct {
	Regions   []model.Region              `json:"regions"`
	Locations map[string][]locationChoice `json:"locations"`
}

func locationOptions() filterOptions {
	opts := filterOptions{Locations: make(map[string][]locationChoice)}
	for _, entry := range region.All() {
		opts.Regions = append(opts.Regions, model.Region{ID: entry.ID, Name: entry.Name})
	}
	for _, station := range metro.AllStations() {
		opts.Locations["station"] = append(opts.Locations["station"], locationChoice{Value: station, Label: station})
	}
	for _, line := range metro.Lines {
		opts.Locations["line"] = append(opts.Locations["line"], locationChoice{Value: line.Number, Label: line.Name + " (" + line.Number + ")"})
	}
	for _, district := range metro.Districts {
		opts.Locations["district"] = append(opts.Locations["district"], locationChoice{Value: district.Name, Label: district.Name})
	}
	for _, okrug := range metro.Okrugs {
		opts.Locations["okrug"] = append(opts.Locations["okrug"], locationChoice{Value: okrug.Name, Label: okrug.Name})
	}
	return opts
}

func readFilter(q url.Values) (db.AdminFilter, error) {
	var f db.AdminFilter
	for key, target := range map[string]*int{"page": &f.Page, "region": &f.Region} {
		if err := integer(q, key, target); err != nil {
			return f, err
		}
	}
	for key, target := range map[string]*int64{
		"min_price": &f.MinPrice, "max_price": &f.MaxPrice, "min_score": &f.MinScore,
		"min_metro_rank": &f.MinUndergroundPlace, "min_floor": &f.MinFloor, "max_floor": &f.MaxFloor,
	} {
		if raw := q.Get(key); raw != "" {
			v, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || v < 0 {
				return f, fmt.Errorf("некорректное число в фильтре %s", key)
			}
			*target = v
		}
	}
	for key, target := range map[string]*float64{
		"min_area": &f.MinArea, "max_area": &f.MaxArea, "min_kitchen": &f.MinKitchenArea, "min_ceiling": &f.MinCeilingHeight,
	} {
		if raw := q.Get(key); raw != "" {
			v, err := strconv.ParseFloat(raw, 64)
			if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				return f, fmt.Errorf("некорректное число в фильтре %s", key)
			}
			*target = v
		}
	}
	if (f.MaxPrice > 0 && f.MinPrice > f.MaxPrice) || (f.MaxArea > 0 && f.MinArea > f.MaxArea) ||
		(f.MaxFloor > 0 && f.MinFloor > f.MaxFloor) {
		return f, fmt.Errorf("нижняя граница фильтра больше верхней")
	}
	for _, value := range q["rooms"] {
		room, err := strconv.ParseInt(value, 10, 64)
		if err != nil || room < 0 || room > 20 {
			return f, fmt.Errorf("некорректное число комнат")
		}
		f.Rooms = append(f.Rooms, room)
	}
	f.DealType, f.Source, f.Sort = q.Get("deal_type"), q.Get("source"), q.Get("sort")
	f.MinRenovation, f.BathroomType = q.Get("renovation"), q.Get("bathroom")
	if !oneOf(f.DealType, "", "rent", "sale") || !oneOf(f.Source, "", "cian", "yandex", "other") ||
		!oneOf(f.Sort, "", "newest", "oldest", "price_asc", "price_desc", "score_asc", "score_desc") ||
		!oneOf(f.MinRenovation, "", "cosmetic", "euro", "design") ||
		!oneOf(f.BathroomType, "", "separated", "combined") {
		return f, fmt.Errorf("неизвестный вариант фильтра или сортировки")
	}
	f.ChildrenRequired = q.Get("children") == "1"
	f.PetsRequired = q.Get("pets") == "1"
	f.DishwasherRequired = q.Get("dishwasher") == "1"
	f.ConditionerRequired = q.Get("conditioner") == "1"
	f.BalconyRequired = q.Get("balcony") == "1"
	f.RoomsMore = q.Get("rooms_more") == "1"
	locations := q["location"]
	if len(locations) > 0 {
		if f.Region != 1 {
			return f, fmt.Errorf("фильтры по метро и районам доступны для Москвы")
		}
		switch q.Get("location_type") {
		case "station":
			f.MetroStations = locations
		case "line":
			f.MetroStations = metro.StationsForLines(locations)
		case "okrug":
			f.MetroStations = metro.StationsForOkrugs(locations)
		case "district":
			f.MetroStations = metro.StationsForDistricts(locations)
		}
		if len(f.MetroStations) == 0 {
			return f, fmt.Errorf("неизвестный район, округ, линия или станция")
		}
	}
	return f, nil
}

func integer(q url.Values, key string, dest *int) error {
	raw := q.Get(key)
	if raw == "" {
		return nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return fmt.Errorf("некорректное число в фильтре %s", key)
	}
	*dest = v
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if item == value {
			return true
		}
	}
	return false
}
