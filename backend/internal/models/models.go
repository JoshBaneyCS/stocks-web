package models

import (
	"time"
)

// ─── Existing tables (read-only, managed by other services) ──────────

// Company maps to the existing `companies` table.
type Company struct {
	ID        int        `json:"id"`
	Symbol    string     `json:"symbol"`
	Name      *string    `json:"name"`
	Exchange  *string    `json:"exchange"`
	Sector    *string    `json:"sector"`
	Industry  *string    `json:"industry"`
	MarketCap *float64   `json:"market_cap"`
	Week52Hi  *float64   `json:"week52_high"`
	Week52Lo  *float64   `json:"week52_low"`
	PrevClose *float64   `json:"prev_close"`
	TodaysHi  *float64   `json:"todays_high"`
	TodaysLo  *float64   `json:"todays_low"`
	UpdatedAt *time.Time `json:"updated_at"`
}

// CompanyListItem is a lighter projection for the stock list endpoint.
type CompanyListItem struct {
	ID        int      `json:"id"`
	Symbol    string   `json:"symbol"`
	Name      *string  `json:"name"`
	Exchange  *string  `json:"exchange"`
	Sector    *string  `json:"sector"`
	Industry  *string  `json:"industry"`
	MarketCap *float64 `json:"market_cap"`
}

// PriceBar maps to the existing `price_bars` table.
type PriceBar struct {
	ID        int       `json:"id"`
	CompanyID int       `json:"company_id"`
	Timestamp time.Time `json:"ts"`
	Interval  string    `json:"interval"` // "1d" or "1min"
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
}

// PricePoint is a slim projection for chart data (no ID/company_id).
type PricePoint struct {
	Timestamp time.Time `json:"ts"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
}

// NewsArticle maps to the existing `news_articles` table.
type NewsArticle struct {
	ID          string     `json:"id"` // UUID
	Provider    string     `json:"provider"`
	SourceName  *string    `json:"source_name"`
	URLOriginal *string    `json:"url"`
	Title       *string    `json:"title"`
	Summary     *string    `json:"summary"`
	PublishedAt *time.Time `json:"published_at"`
}

// NewsMention maps to the existing `news_mentions` table.
type NewsMention struct {
	ID        int    `json:"id"`
	ArticleID string `json:"article_id"`
	CompanyID *int   `json:"company_id"`
	Symbol    string `json:"symbol"`
}

// ─── New tables (managed by this service via migrations) ─────────────

// User represents an authenticated user account.
type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // never serialize
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserPublic is the safe projection returned by API endpoints.
type UserPublic struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	CreatedAt time.Time `json:"created_at"`
}

// ToPublic converts a User to a UserPublic (strips password hash).
func (u *User) ToPublic() UserPublic {
	return UserPublic{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		CreatedAt: u.CreatedAt,
	}
}

// ReferralCode controls invite-only signups.
type ReferralCode struct {
	ID         int       `json:"id"`
	Code       string    `json:"code"`
	Status     string    `json:"status"` // "active" or "disabled"
	UsageLimit *int      `json:"usage_limit"`
	UsedCount  int       `json:"used_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// UserFavorite links a user to a favorited company.
type UserFavorite struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	CompanyID int       `json:"company_id"`
	CreatedAt time.Time `json:"created_at"`
}

// RefreshToken tracks active refresh tokens per user.
type RefreshToken struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	TokenHash string    `json:"-"` // SHA-256 of the raw token
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// ─── API request/response types ──────────────────────────────────────

// SignupRequest is the payload for POST /api/auth/signup.
type SignupRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	ReferralCode string `json:"referral_code"`
}

// LoginRequest is the payload for POST /api/auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse is returned on successful login/signup.
type AuthResponse struct {
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time  `json:"expires_at"`
	User         UserPublic `json:"user"`
}

// FavoritesUpdateRequest is the payload for PUT /api/favorites.
type FavoritesUpdateRequest struct {
	CompanyIDs []int `json:"company_ids"`
}

// StockListResponse wraps paginated stock results.
type StockListResponse struct {
	Stocks     []CompanyListItem `json:"stocks"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
}

// StockDetailResponse is the full detail for a single stock.
type StockDetailResponse struct {
	Company    Company     `json:"company"`
	LatestBar  *PricePoint `json:"latest_bar"`
	IsFavorite bool        `json:"is_favorite"`
}

// MarketStatusResponse describes current market state.
type MarketStatusResponse struct {
	IsOpen      bool       `json:"is_open"`
	CurrentTime time.Time  `json:"current_time"`
	NextOpen    *time.Time `json:"next_open,omitempty"`
	NextClose   *time.Time `json:"next_close,omitempty"`
	Timezone    string     `json:"timezone"`
}

// CreateReferralCodeRequest is the admin payload for creating referral codes.
type CreateReferralCodeRequest struct {
	Code       string `json:"code"`
	UsageLimit *int   `json:"usage_limit,omitempty"`
}
