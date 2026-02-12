package models

import "time"

// ---- User ----

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserPublic is the safe representation of a user for API responses.
type UserPublic struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	CreatedAt time.Time `json:"created_at"`
}

// Public converts a User to a UserPublic (omitting sensitive fields).
func (u *User) Public() UserPublic {
	return UserPublic{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		CreatedAt: u.CreatedAt,
	}
}

// ---- Instruments ----

type Instrument struct {
	ID         int     `json:"id"`
	Symbol     string  `json:"symbol"`
	Name       *string `json:"name"`
	Exchange   *string `json:"exchange"`
	Currency   *string `json:"currency"`
	Country    *string `json:"country"`
	AssetClass string  `json:"asset_class"`
	IsActive   bool    `json:"is_active"`
}

type InstrumentListItem struct {
	Instrument
	LastPrice  *float64 `json:"last_price"`
	MarketCap  *float64 `json:"market_cap"`
	Sector     *string  `json:"sector"`
	Industry   *string  `json:"industry"`
	IsFavorite bool     `json:"is_favorite"`
}

type InstrumentDetail struct {
	Instrument
	LastPrice   *float64        `json:"last_price"`
	MarketCap   *float64        `json:"market_cap"`
	Profile     *CompanyProfile `json:"profile"`
	LatestQuote *Quote          `json:"latest_quote"`
}

// ---- Company Profile ----

type CompanyProfile struct {
	MarketCap *float64 `json:"market_cap"`
	Sector    *string  `json:"sector"`
	Industry  *string  `json:"industry"`
	Exchange  *string  `json:"exchange"`
	Country   *string  `json:"country"`
	Currency  *string  `json:"currency"`
}

// ---- Price & Quote ----

type PriceBar struct {
	Timestamp time.Time `json:"ts"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    int64     `json:"volume"`
	AdjClose  *float64  `json:"adj_close,omitempty"`
}

type Quote struct {
	Timestamp time.Time `json:"ts"`
	LastPrice *float64  `json:"last_price"`
	Bid       *float64  `json:"bid"`
	Ask       *float64  `json:"ask"`
	Volume    *int64    `json:"volume"`
	Source    *string   `json:"source"`
}

// ---- Fundamentals ----

type FundamentalsRow struct {
	PeriodEndDate   string   `json:"period_end_date"`
	CalendarYear    *int     `json:"calendar_year"`
	Period          *string  `json:"period"`
	Revenue         *float64 `json:"revenue"`
	GrossProfit     *float64 `json:"gross_profit"`
	OperatingIncome *float64 `json:"operating_income"`
	NetIncome       *float64 `json:"net_income"`
	EPS             *float64 `json:"eps"`
}

// ---- Auth Requests / Responses ----

type SignupRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	ReferralCode string `json:"referral_code"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	AccessToken string     `json:"access_token"`
	User        UserPublic `json:"user"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// ---- Favorites ----

type FavoritesUpdateRequest struct {
	InstrumentIDs []int `json:"instrument_ids"`
}

// ---- Market Status ----

type MarketStatus struct {
	IsOpen    bool       `json:"is_open"`
	NextOpen  *time.Time `json:"next_open"`
	NextClose *time.Time `json:"next_close"`
	Message   string     `json:"message"`
}

// ---- Pagination ----

type PaginatedResponse[T any] struct {
	Data       []T `json:"data"`
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalCount int `json:"total_count"`
	TotalPages int `json:"total_pages"`
}

// ---- Dashboard ----

type DashboardResponse struct {
	Favorites    []InstrumentListItem `json:"favorites"`
	MarketStatus MarketStatus         `json:"market_status"`
}

// ---- Admin ----

type CreateReferralCodeRequest struct {
	Code       string `json:"code"`
	UsageLimit *int   `json:"usage_limit"`
}

type ReferralCode struct {
	Code       string    `json:"code"`
	IsActive   bool      `json:"is_active"`
	UsageLimit *int      `json:"usage_limit"`
	UsedCount  int       `json:"used_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// ---- SSE Events ----

type PriceEvent struct {
	Symbol    string   `json:"symbol"`
	LastPrice *float64 `json:"last_price"`
	Bid       *float64 `json:"bid"`
	Ask       *float64 `json:"ask"`
	Volume    *int64   `json:"volume"`
	Timestamp string   `json:"ts"`
}
