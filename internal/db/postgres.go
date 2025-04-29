package db

import (
	"awesomeProject/Diets_Bot/internal/models"
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

type PostgresDB struct {
	pool *pgxpool.Pool
}

func NewPostgresDB(cfg struct {
	Host         string
	Port         string
	User         string
	Password     string
	DBName       string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
	ConnLifetime time.Duration
}) (*PostgresDB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s pool_max_conns=%d",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode, cfg.MaxOpenConns,
	)

	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DB connection string: %w", err)
	}

	// Set connection pool parameters
	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.ConnLifetime
	poolConfig.MaxConnIdleTime = 15 * time.Minute

	// Connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.ConnectConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Verify connection works
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresDB{pool: pool}, nil
}

func (db *PostgresDB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}
func (db *PostgresDB) GetPaymentByStripeID(ctx context.Context, stripePaymentID string) (*models.Payment, error) {
	query := `
        SELECT id, user_id, amount, currency, stripe_payment_id, status, created_at, updated_at
        FROM payments
        WHERE stripe_payment_id = $1
    `

	var payment models.Payment
	err := db.pool.QueryRow(ctx, query, stripePaymentID).Scan(
		&payment.ID, &payment.UserID, &payment.Amount, &payment.Currency,
		&payment.StripePaymentID, &payment.Status,
		&payment.CreatedAt, &payment.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get payment by Stripe ID: %w", err)
	}

	return &payment, nil
}

func (db *PostgresDB) SaveUser(ctx context.Context, user *models.User) error {
	query := `
        INSERT INTO users (telegram_id, chat_id, username, gender, height, weight, goal)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (telegram_id) DO UPDATE
        SET gender = $4, height = $5, weight = $6, goal = $7, updated_at = NOW()
        RETURNING id
    `

	err := db.pool.QueryRow(ctx, query,
		user.TelegramID, user.ChatID, user.Username,
		user.Gender, user.Height, user.Weight, user.Goal,
	).Scan(&user.ID)

	return err
}

func (db *PostgresDB) GetUser(ctx context.Context, telegramID int64) (*models.User, error) {
	query := `
        SELECT id, telegram_id, chat_id, username, gender, height, weight, goal, created_at, updated_at
        FROM users
        WHERE telegram_id = $1
    `

	var user models.User
	err := db.pool.QueryRow(ctx, query, telegramID).Scan(
		&user.ID, &user.TelegramID, &user.ChatID, &user.Username,
		&user.Gender, &user.Height, &user.Weight, &user.Goal,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (db *PostgresDB) SavePayment(ctx context.Context, payment *models.Payment) error {
	query := `
        INSERT INTO payments (user_id, amount, currency, stripe_payment_id, status)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id
    `

	err := db.pool.QueryRow(ctx, query,
		payment.UserID, payment.Amount, payment.Currency,
		payment.StripePaymentID, payment.Status,
	).Scan(&payment.ID)

	return err
}

func (db *PostgresDB) UpdatePaymentStatus(ctx context.Context, stripePaymentID string, status string) error {
	query := `
        UPDATE payments
        SET status = $2, updated_at = NOW()
        WHERE stripe_payment_id = $1
    `

	_, err := db.pool.Exec(ctx, query, stripePaymentID, status)
	return err
}

func (db *PostgresDB) SaveDietPlan(ctx context.Context, plan *models.DietPlan) error {
	query := `
        INSERT INTO diet_plans (user_id, payment_id, plan_text)
        VALUES ($1, $2, $3)
        RETURNING id
    `

	err := db.pool.QueryRow(ctx, query,
		plan.UserID, plan.PaymentID, plan.PlanText,
	).Scan(&plan.ID)

	return err
}

func (db *PostgresDB) GetDietPlan(ctx context.Context, userID int64) (*models.DietPlan, error) {
	query := `
        SELECT id, user_id, payment_id, plan_text, created_at
        FROM diet_plans
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT 1
    `

	var plan models.DietPlan
	err := db.pool.QueryRow(ctx, query, userID).Scan(
		&plan.ID, &plan.UserID, &plan.PaymentID, &plan.PlanText, &plan.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &plan, nil
}
