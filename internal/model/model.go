// Package model defines the JSON-facing types served to the iOS app. Field
// names are snake_case to match standard REST/JSON conventions on the wire;
// the Swift client decodes them with .convertFromSnakeCase.
package model

import "time"

// Flat mirrors a flats_history row, trimmed to what the app displays.
type Flat struct {
	ID                       int       `json:"id"`
	Link                     string    `json:"link"`
	ParsedAt                 time.Time `json:"parsed_at"`
	Region                   int       `json:"region"`
	DealType                 string    `json:"deal_type"`
	Price                    int       `json:"price"`
	Score                    int       `json:"score"`
	UndergroundScore         float64   `json:"underground_score"`
	UndergroundPlace         int       `json:"underground_place"`
	UndergroundDistanceInfo  string    `json:"underground_distance_info"`
	UndergroundStations      []string  `json:"underground_stations"`
	PhotoURLs                []string  `json:"photo_urls"`
	PhotoThumbnailURLs       []string  `json:"photo_thumbnail_urls"`
	RoomNumber               int       `json:"room_number"`
	TotalArea                float64   `json:"total_area"`
	LivingArea               float64   `json:"living_area"`
	KitchenArea              float64   `json:"kitchen_area"`
	Floor                    int       `json:"floor"`
	MaxFloor                 int       `json:"max_floor"`
	Deposit                  int       `json:"deposit"`
	DepositMonths            int       `json:"deposit_months"`
	Comission                int       `json:"comission"`
	Renovation               string    `json:"renovation"`
	IsApartments             bool      `json:"is_apartments"`
	LoggiaCount              int       `json:"loggia_count"`
	BalconyCount             int       `json:"balcony_count"`
	WindowsView              string    `json:"windows_view"`
	SeparatedBathroomCount   int       `json:"separated_bathroom_count"`
	CombinedBathroomCount    int       `json:"combined_bathroom_count"`
	HasDishwasher            bool      `json:"has_dishwasher"`
	HasConditioner           bool      `json:"has_conditioner"`
	ChildrenAllowed          bool      `json:"children_allowed"`
	PetsAllowed              bool      `json:"pets_allowed"`
	CeilingHeight            float64   `json:"ceiling_height"`
	BuildingEntrancesNumber  int       `json:"building_entrances_number"`
	BuildingApartmentsNumber int       `json:"building_apartments_number"`
	BuildingElevatorsNumber  int       `json:"building_elevators_number"`
	Latitude                 float64   `json:"latitude"`
	Longitude                float64   `json:"longitude"`

	SaleType                  string `json:"sale_type"`
	MortgageAllowed           bool   `json:"mortgage_allowed"`
	IsNewBuilding             bool   `json:"is_new_building"`
	NewBuildingName           string `json:"new_building_name"`
	IsByHomeowner             bool   `json:"is_by_homeowner"`
	DemolishedInMoscowProgram bool   `json:"demolished_in_moscow_program"`
}

// ScoringParams holds the 20 customizable scoring multipliers a subscriber
// may override; nil/omitted means "use default scoring".
type ScoringParams struct {
	AllArea            float64 `json:"all_area"`
	KitchenArea        float64 `json:"kitchen_area"`
	Pets               float64 `json:"pets"`
	Dishwasher         float64 `json:"dishwasher"`
	Conditioner        float64 `json:"conditioner"`
	Apartments         float64 `json:"apartments"`
	TwoRoom            float64 `json:"two_room"`
	ThreeRoom          float64 `json:"three_room"`
	FourRoom           float64 `json:"four_room"`
	AdditionalRooms    float64 `json:"additional_rooms"`
	WindowsYard        float64 `json:"windows_yard"`
	WindowsStreet      float64 `json:"windows_street"`
	WindowsBoth        float64 `json:"windows_both"`
	RenovationDesign   float64 `json:"renovation_design"`
	RenovationEuro     float64 `json:"renovation_euro"`
	RenovationCosmetic float64 `json:"renovation_cosmetic"`
	BathroomSeparated  float64 `json:"bathroom_separated"`
	Balcony            float64 `json:"balcony"`
	Loggia             float64 `json:"loggia"`
	Underground        float64 `json:"underground"`
}

// Subscription is a saved search: a named filter set the app polls for
// matching flats and can badge with an unseen count.
type Subscription struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`

	DealType string `json:"deal_type"`
	Region   int    `json:"region"`

	MinPrice int64   `json:"min_price"`
	MaxPrice int64   `json:"max_price"`
	MinArea  float64 `json:"min_area"`
	MaxArea  float64 `json:"max_area"`
	Rooms    []int64 `json:"rooms"`
	MinScore int64   `json:"min_score"`

	MetroStations    []string `json:"metro_stations"`
	MetroFilterLabel string   `json:"metro_filter_label"`

	MinUndergroundPlace int64   `json:"min_underground_place"`
	MinKitchenArea      float64 `json:"min_kitchen_area"`
	MinFloor            int64   `json:"min_floor"`
	MaxFloor            int64   `json:"max_floor"`
	MinCeilingHeight    float64 `json:"min_ceiling_height"`
	ChildrenRequired    bool    `json:"children_required"`
	PetsRequired        bool    `json:"pets_required"`
	DishwasherRequired  bool    `json:"dishwasher_required"`
	ConditionerRequired bool    `json:"conditioner_required"`
	MinRenovation       string  `json:"min_renovation"`
	BalconyRequired     bool    `json:"balcony_required"`
	BathroomType        string  `json:"bathroom_type"`

	Scoring *ScoringParams `json:"scoring,omitempty"`

	// NewCount is the number of matching flats parsed since LastViewedAt;
	// populated only in the subscription list response.
	NewCount int `json:"new_count,omitempty"`
}

type Region struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type MetroLine struct {
	Number   string   `json:"number"`
	Name     string   `json:"name"`
	Emoji    string   `json:"emoji"`
	Stations []string `json:"stations"`
}

type Okrug struct {
	Name     string   `json:"name"`
	Stations []string `json:"stations"`
}

type User struct {
	ID          int       `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}
