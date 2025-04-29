// internal/models/user.go
package models

import (
	"time"
)

type User struct {
	ID         int64     `json:"id"`
	TelegramID int64     `json:"telegram_id"`
	ChatID     int64     `json:"chat_id"`
	Username   string    `json:"username"`
	Gender     string    `json:"gender"`
	Height     int       `json:"height"`
	Weight     int       `json:"weight"`
	Goal       string    `json:"goal"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Payment struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"user_id"`
	Amount          int       `json:"amount"`
	Currency        string    `json:"currency"`
	StripePaymentID string    `json:"stripe_payment_id"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type DietPlan struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	PaymentID int64     `json:"payment_id"`
	PlanText  string    `json:"plan_text"`
	CreatedAt time.Time `json:"created_at"`
}

type UserState struct {
	TelegramID      int64                  `json:"telegram_id"`
	CurrentState    string                 `json:"current_state"`
	TemporaryData   map[string]interface{} `json:"temporary_data"`
	StripeSessionID string                 `json:"stripe_session_id"`
}
