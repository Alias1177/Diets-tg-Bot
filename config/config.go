// internal/config/config.go
package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Telegram struct {
		Token string
	}
	DB struct {
		Host         string
		Port         string
		User         string
		Password     string
		DBName       string
		SSLMode      string
		MaxOpenConns int
		MaxIdleConns int
		ConnLifetime time.Duration
	}
	Stripe struct {
		SecretKey  string
		PublicKey  string
		WebhookKey string
		ProductID  string
		PriceID    string
	}
	GPT struct {
		APIKey string
		Model  string
	}
	Server struct {
		Port string
	}
	ShutdownTimeout time.Duration
}

// Load loads the configuration
func Load() (*Config, error) {
	_ = godotenv.Load()

	// Print current working directory for debugging
	dir, _ := os.Getwd()
	fmt.Printf("Current working directory: %s\n", dir)

	// Create a new viper instance
	v := viper.New()

	// Set the config name (without extension)
	v.SetConfigName("config")

	// Add supported config file types
	v.SetConfigType("yaml")
	v.SetConfigType("json")

	// Add paths where to look for the config file
	v.AddConfigPath(".")               // Look in current directory
	v.AddConfigPath("./config")        // Look in config subdirectory
	v.AddConfigPath("../config")       // Look in sibling config directory
	v.AddConfigPath("$HOME/.diet-bot") // Look in home directory

	// Set default values
	v.SetDefault("ShutdownTimeout", 10*time.Second)
	v.SetDefault("GPT.Model", "gpt-4")
	v.SetDefault("Server.Port", "8080")
	v.SetDefault("DB.MaxOpenConns", 20)
	v.SetDefault("DB.MaxIdleConns", 10)
	v.SetDefault("DB.ConnLifetime", 5*time.Minute)

	// Enable environment variables to override config values
	v.AutomaticEnv()

	// Try to read config file
	err := v.ReadInConfig()

	// If can't find config file, try to create one with default values
	if err != nil {
		fmt.Printf("Config file not found: %v\n", err)
		fmt.Println("Creating config file with environment variables...")

		// Create a minimal config with environment variables
		cfg := &Config{}

		// Set values from environment variables
		cfg.Telegram.Token = os.Getenv("TELEGRAM_TOKEN")
		cfg.DB.Host = getEnvOr("DB_HOST", "localhost")
		cfg.DB.Port = getEnvOr("DB_PORT", "5432")
		cfg.DB.User = getEnvOr("DB_USER", "postgres")
		cfg.DB.Password = getEnvOr("DB_PASSWORD", "postgres")
		cfg.DB.DBName = getEnvOr("DB_NAME", "fitness_bot")
		cfg.DB.SSLMode = getEnvOr("DB_SSL_MODE", "disable")
		cfg.Stripe.SecretKey = os.Getenv("STRIPE_SECRET_KEY")
		cfg.Stripe.PublicKey = os.Getenv("STRIPE_PUBLIC_KEY")
		cfg.Stripe.WebhookKey = os.Getenv("STRIPE_WEBHOOK_KEY")
		cfg.Stripe.ProductID = os.Getenv("STRIPE_PRODUCT_ID")
		cfg.Stripe.PriceID = os.Getenv("STRIPE_PRICE_ID")
		cfg.GPT.APIKey = os.Getenv("GPT_API_KEY")
		cfg.GPT.Model = getEnvOr("GPT_MODEL", "gpt-4")
		cfg.Server.Port = getEnvOr("SERVER_PORT", "8080")

		return cfg, nil
	}

	// Process any ${ENV_VAR} syntax in the config values
	for _, key := range v.AllKeys() {
		value := v.GetString(key)
		if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
			envVar := strings.TrimPrefix(strings.TrimSuffix(value, "}"), "${")
			envValue := os.Getenv(envVar)
			if envValue != "" {
				v.Set(key, envValue)
			}
		}
	}

	// Unmarshal config to struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &cfg, nil
}

// Helper function to get environment variable with default value
func getEnvOr(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
