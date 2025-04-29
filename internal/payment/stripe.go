// internal/payment/stripe.go
package payment

import (
	"fmt"
	"strconv"

	"github.com/stripe/stripe-go/v72/checkout/session"
	"github.com/stripe/stripe-go/v72/webhook"
)

type StripeClient struct {
	secretKey     string
	publicKey     string
	webhookSecret string
	priceID       string
	productID     string
}

func NewStripeClient(config struct {
	SecretKey  string
	PublicKey  string
	WebhookKey string
	ProductID  string
	PriceID    string
}) *StripeClient {
	// Set the secret key for backend operations
	stripe.Key = config.SecretKey

	return &StripeClient{
		secretKey:     config.SecretKey,
		publicKey:     config.PublicKey,
		webhookSecret: config.WebhookKey,
		priceID:       config.PriceID,
		productID:     config.ProductID,
	}
}

func (s *StripeClient) GetWebhookSecret() string {
	return s.webhookSecret
}

func (s *StripeClient) GetPriceID() string {
	return s.priceID
}

// Modified to return both session ID and URL
func (s *StripeClient) CreateCheckoutSession(userID int64, successURL, cancelURL string) (string, string, error) {
	// Ensure we're using the secret key for API operations
	if stripe.Key != s.secretKey {
		stripe.Key = s.secretKey
	}

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(s.priceID),
				Quantity: stripe.Int64(1),
			},
		},
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		ClientReferenceID: stripe.String(strconv.FormatInt(userID, 10)),
	}

	sess, err := session.New(params)
	if err != nil {
		return "", "", fmt.Errorf("failed to create checkout session: %v", err)
	}

	return sess.ID, sess.URL, nil
}

func (s *StripeClient) VerifyWebhookSignature(payload []byte, sig string, webhookSecret string) (stripe.Event, error) {
	if webhookSecret == "" {
		return stripe.Event{}, fmt.Errorf("webhook secret is not configured")
	}
	return webhook.ConstructEvent(payload, sig, webhookSecret)
}
