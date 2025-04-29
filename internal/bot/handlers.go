package bot

import (
	"encoding/json"
	"github.com/stripe/stripe-go/v72"
	"io"
	"net/http"
	"strconv"
)

func (t *TelegramBot) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.logger.Error("Failed to read webhook body", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Get webhook secret
	webhookSecret := t.stripeClient.GetWebhookSecret()
	if webhookSecret == "" {
		t.logger.Error("Webhook secret is not configured")
		http.Error(w, "Webhook not configured", http.StatusInternalServerError)
		return
	}

	// Verify Stripe signature
	signature := r.Header.Get("Stripe-Signature")
	if signature == "" {
		t.logger.Error("Missing Stripe signature header")
		http.Error(w, "Missing signature", http.StatusBadRequest)
		return
	}

	event, err := t.stripeClient.VerifyWebhookSignature(body, signature, webhookSecret)
	if err != nil {
		t.logger.Error("Failed to verify webhook signature", err)
		http.Error(w, "Invalid signature", http.StatusBadRequest)
		return
	}

	// Process different event types
	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		err := json.Unmarshal(event.Data.Raw, &session)
		if err != nil {
			t.logger.Error("Failed to parse checkout session", err)
			http.Error(w, "Failed to parse event data", http.StatusBadRequest)
			return
		}

		// Validate client reference ID (user ID)
		if session.ClientReferenceID == "" {
			t.logger.Error("Missing client reference ID", "sessionID", session.ID)
			http.Error(w, "Missing client reference ID", http.StatusBadRequest)
			return
		}

		userID, err := strconv.ParseInt(session.ClientReferenceID, 10, 64)
		if err != nil {
			t.logger.Error("Invalid client reference ID", err, "value", session.ClientReferenceID)
			http.Error(w, "Invalid client reference ID", http.StatusBadRequest)
			return
		}

		// Check payment intent exists
		if session.PaymentIntent == nil {
			t.logger.Error("Payment intent is nil", "sessionID", session.ID)
			http.Error(w, "Payment intent is nil", http.StatusBadRequest)
			return
		}

		// Process payment success in background to avoid webhook timeout
		go t.handlePaymentSuccess(userID, session.PaymentIntent.ID)
		t.logger.Info("Payment processing started", "userID", userID, "paymentID", session.PaymentIntent.ID)

	case "payment_intent.succeeded":
		// Log payment intent success
		var intent stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &intent); err != nil {
			t.logger.Error("Failed to parse payment intent", err)
			break
		}
		t.logger.Info("Payment intent succeeded", "paymentID", intent.ID)

	case "payment_intent.payment_failed":
		// Log payment failure
		var intent stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &intent); err != nil {
			t.logger.Error("Failed to parse payment intent", err)
			break
		}
		t.logger.Error("Payment failed", "paymentID", intent.ID, "error", intent.LastPaymentError)
	}

	// Respond with 200 OK to acknowledge receipt
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Webhook received"))
}
