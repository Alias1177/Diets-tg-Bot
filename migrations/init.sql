-- migrations/init.sql
CREATE TABLE IF NOT EXISTS users (
                                     id SERIAL PRIMARY KEY,
                                     telegram_id BIGINT UNIQUE NOT NULL,
                                     chat_id BIGINT NOT NULL,
                                     username VARCHAR(255),
                                     gender VARCHAR(50) NOT NULL,
                                     height INTEGER NOT NULL,
                                     weight INTEGER NOT NULL,
                                     goal VARCHAR(50) NOT NULL,
                                     created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                     updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS payments (
                                        id SERIAL PRIMARY KEY,
                                        user_id INTEGER REFERENCES users(id),
                                        amount INTEGER NOT NULL,
                                        currency VARCHAR(10) NOT NULL,
                                        stripe_payment_id VARCHAR(255) UNIQUE NOT NULL,
                                        status VARCHAR(50) NOT NULL,
                                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS diet_plans (
                                          id SERIAL PRIMARY KEY,
                                          user_id INTEGER REFERENCES users(id),
                                          payment_id INTEGER REFERENCES payments(id),
                                          plan_text TEXT NOT NULL,
                                          created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);
CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id);
CREATE INDEX IF NOT EXISTS idx_payments_stripe_payment_id ON payments(stripe_payment_id);
CREATE INDEX IF NOT EXISTS idx_diet_plans_user_id ON diet_plans(user_id);