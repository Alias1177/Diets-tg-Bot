package bot

import (
	"context"
	"diet-bot/internal/db"
	"diet-bot/internal/gpt"
	"diet-bot/internal/models"
	"diet-bot/internal/payment"
	"diet-bot/pkg/logger"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"strconv"
	"sync"
	"time"
)

const (
	StateStart      = "start"
	StateGender     = "gender"
	StateHeight     = "height"
	StateWeight     = "weight"
	StateGoal       = "goal"
	StateConfirm    = "confirm"
	StatePayment    = "payment"
	StateProcessing = "processing"
	StateComplete   = "complete"
)

type TelegramBot struct {
	bot          *tgbotapi.BotAPI
	db           *db.PostgresDB
	stripeClient *payment.StripeClient
	gptClient    *gpt.Client
	logger       *logger.Logger
	userStates   map[int64]*models.UserState
	stateMutex   sync.RWMutex
	callbackURL  string
}

func NewTelegramBot(token string, db *db.PostgresDB, stripeClient *payment.StripeClient, gptClient *gpt.Client, logger *logger.Logger) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	// Enable debug mode for more detailed logging
	bot.Debug = true

	logger.Info("Authorized on Telegram", "username", bot.Self.UserName)

	return &TelegramBot{
		bot:          bot,
		db:           db,
		stripeClient: stripeClient,
		gptClient:    gptClient,
		logger:       logger,
		userStates:   make(map[int64]*models.UserState),
		stateMutex:   sync.RWMutex{},
		callbackURL:  fmt.Sprintf("https://t.me/%s", bot.Self.UserName),
	}, nil
}

// Start begins receiving updates from Telegram via polling
func (t *TelegramBot) Start(ctx context.Context) error {
	// First, remove any existing webhook to ensure we can use polling
	t.logger.Info("Removing any existing webhook")
	_, err := t.bot.Request(tgbotapi.DeleteWebhookConfig{
		DropPendingUpdates: true,
	})
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	t.logger.Info("Webhook removed, starting polling for updates")

	// Configure update channel
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	// Start receiving updates
	updates := t.bot.GetUpdatesChan(updateConfig)

	t.logger.Info("Started receiving Telegram updates")

	// Handle updates in a goroutine
	go t.handleUpdates(ctx, updates)

	return nil
}

// handleUpdates processes incoming updates from Telegram
func (t *TelegramBot) handleUpdates(ctx context.Context, updates tgbotapi.UpdatesChannel) {
	for update := range updates {
		go func(update tgbotapi.Update) {
			// Add recovery for panics
			defer func() {
				if r := recover(); r != nil {
					t.logger.Error("Recovered from panic while processing update", "error", r)
				}
			}()

			t.logger.Info("Received update", "update_id", update.UpdateID)

			if update.Message != nil {
				// Process message
				t.logger.Info("Received message",
					"chat_id", update.Message.Chat.ID,
					"from", update.Message.From.UserName,
					"text", update.Message.Text)

				if update.Message.IsCommand() {
					// Handle commands
					t.handleCommand(update.Message)
				} else {
					// Handle regular messages based on user state
					t.handleMessage(update.Message)
				}
			} else if update.CallbackQuery != nil {
				// Handle callback queries (e.g., from inline buttons)
				t.handleCallbackQuery(update.CallbackQuery)
			}
		}(update)
	}
}

// handleCommand processes bot commands
func (t *TelegramBot) handleCommand(message *tgbotapi.Message) {
	command := message.Command()
	chatID := message.Chat.ID
	userID := message.From.ID

	t.logger.Info("Handling command", "command", command, "user_id", userID)

	switch command {
	case "start":
		// Check if this is a payment callback
		if message.CommandArguments() == "payment_success" {
			// Handle successful payment
			t.stateMutex.RLock()
			state, exists := t.userStates[userID]
			t.stateMutex.RUnlock()

			if exists && state.StripeSessionID != "" {
				// Send confirmation message
				msg := tgbotapi.NewMessage(chatID, "Спасибо за оплату! Ваш персонализированный план питания будет готов в ближайшее время.")
				t.bot.Send(msg)

				// Process payment asynchronously
				go func() {
					// In a real implementation, you would verify the payment status with Stripe
					// For now, we'll simulate success
					t.handlePaymentSuccess(userID, state.StripeSessionID)
				}()
				return
			}
		} else if message.CommandArguments() == "payment_cancel" {
			// Handle cancelled payment
			msg := tgbotapi.NewMessage(chatID, "Оплата была отменена. Вы можете попробовать снова, используя /start.")
			t.bot.Send(msg)

			// Reset user state
			t.stateMutex.Lock()
			t.userStates[userID] = &models.UserState{
				TelegramID:    userID,
				CurrentState:  StateStart,
				TemporaryData: make(map[string]interface{}),
			}
			t.stateMutex.Unlock()
			return
		}

		// Initialize user state
		t.stateMutex.Lock()
		t.userStates[userID] = &models.UserState{
			TelegramID:    userID,
			CurrentState:  StateGender,
			TemporaryData: make(map[string]interface{}),
		}
		t.stateMutex.Unlock()

		// Send welcome message with gender selection
		replyMarkup := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Мужской"),
				tgbotapi.NewKeyboardButton("Женский"),
			),
		)

		msg := tgbotapi.NewMessage(chatID, "👋 Приветствую! Я помогу вам создать персонализированный план питания. Для начала, укажите ваш пол:")
		msg.ReplyMarkup = replyMarkup

		sent, err := t.bot.Send(msg)
		if err != nil {
			t.logger.Error("Failed to send start message", "error", err)
		} else {
			t.logger.Info("Sent start message", "message_id", sent.MessageID)
		}

	case "help":
		// Send help information
		msg := tgbotapi.NewMessage(chatID, "Я бот для создания персонализированных планов питания. Используйте /start, чтобы начать процесс.")
		_, err := t.bot.Send(msg)
		if err != nil {
			t.logger.Error("Failed to send help message", "error", err)
		}

	default:
		// Unknown command
		msg := tgbotapi.NewMessage(chatID, "Неизвестная команда. Используйте /start для начала работы.")
		_, err := t.bot.Send(msg)
		if err != nil {
			t.logger.Error("Failed to send unknown command message", "error", err)
		}
	}
}

// handleMessage processes regular messages based on user state
func (t *TelegramBot) handleMessage(message *tgbotapi.Message) {
	chatID := message.Chat.ID
	userID := message.From.ID
	text := message.Text

	// Get user state
	t.stateMutex.RLock()
	state, exists := t.userStates[userID]
	t.stateMutex.RUnlock()

	if !exists {
		// User has no state, prompt to start
		msg := tgbotapi.NewMessage(chatID, "Пожалуйста, используйте /start для начала работы с ботом.")
		_, err := t.bot.Send(msg)
		if err != nil {
			t.logger.Error("Failed to send no state message", "error", err)
		}
		return
	}

	t.logger.Info("Processing message based on state",
		"user_id", userID,
		"state", state.CurrentState,
		"text", text)

	// Process based on current state
	switch state.CurrentState {
	case StateGender:
		if text != "Мужской" && text != "Женский" {
			msg := tgbotapi.NewMessage(chatID, "Пожалуйста, выберите пол с помощью кнопок ниже.")
			msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("Мужской"),
					tgbotapi.NewKeyboardButton("Женский"),
				),
			)
			t.bot.Send(msg)
			return
		}

		// Save gender and move to next state
		state.TemporaryData["gender"] = text
		state.CurrentState = StateHeight

		// Ask for height
		msg := tgbotapi.NewMessage(chatID, "Спасибо! Теперь укажите ваш рост в сантиметрах (например, 175):")
		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		t.bot.Send(msg)

	case StateHeight:
		// Try to parse height
		height, err := strconv.Atoi(text)
		if err != nil || height < 50 || height > 250 {
			msg := tgbotapi.NewMessage(chatID, "Пожалуйста, введите корректный рост в сантиметрах (например, 175):")
			t.bot.Send(msg)
			return
		}

		// Save height and move to next state
		state.TemporaryData["height"] = height
		state.CurrentState = StateWeight

		// Ask for weight
		msg := tgbotapi.NewMessage(chatID, "Спасибо! Теперь укажите ваш вес в килограммах (например, 70):")
		t.bot.Send(msg)

	case StateWeight:
		// Try to parse weight
		weight, err := strconv.Atoi(text)
		if err != nil || weight < 30 || weight > 300 {
			msg := tgbotapi.NewMessage(chatID, "Пожалуйста, введите корректный вес в килограммах (например, 70):")
			t.bot.Send(msg)
			return
		}

		// Save weight and move to next state
		state.TemporaryData["weight"] = weight
		state.CurrentState = StateGoal

		// Ask for goal
		msg := tgbotapi.NewMessage(chatID, "Спасибо! Какая у вас цель?")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Снизить вес"),
				tgbotapi.NewKeyboardButton("Поддерживать вес"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Набрать вес"),
			),
		)
		t.bot.Send(msg)

	case StateGoal:
		validGoals := map[string]bool{
			"Снизить вес":      true,
			"Поддерживать вес": true,
			"Набрать вес":      true,
		}

		if !validGoals[text] {
			msg := tgbotapi.NewMessage(chatID, "Пожалуйста, выберите цель с помощью кнопок ниже.")
			msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("Снизить вес"),
					tgbotapi.NewKeyboardButton("Поддерживать вес"),
				),
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("Набрать вес"),
				),
			)
			t.bot.Send(msg)
			return
		}

		// Save goal and move to confirmation
		state.TemporaryData["goal"] = text
		state.CurrentState = StateConfirm

		// Show summary and ask for confirmation
		gender := state.TemporaryData["gender"].(string)
		height := state.TemporaryData["height"].(int)
		weight := state.TemporaryData["weight"].(int)
		goal := state.TemporaryData["goal"].(string)

		summary := fmt.Sprintf("Давайте проверим введенные данные:\n\nПол: %s\nРост: %d см\nВес: %d кг\nЦель: %s\n\nВсё верно?", gender, height, weight, goal)

		msg := tgbotapi.NewMessage(chatID, summary)
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Да, всё верно"),
				tgbotapi.NewKeyboardButton("Нет, изменить"),
			),
		)
		t.bot.Send(msg)

	case StateConfirm:
		if text == "Нет, изменить" {
			// Reset to beginning of form
			state.CurrentState = StateGender
			state.TemporaryData = make(map[string]interface{})

			msg := tgbotapi.NewMessage(chatID, "Давайте начнем заново. Выберите ваш пол:")
			msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("Мужской"),
					tgbotapi.NewKeyboardButton("Женский"),
				),
			)
			t.bot.Send(msg)
			return
		}

		if text != "Да, всё верно" {
			msg := tgbotapi.NewMessage(chatID, "Пожалуйста, выберите один из вариантов ответа.")
			t.bot.Send(msg)
			return
		}

		// Process confirmation and proceed to payment
		ctx := context.Background()
		gender := state.TemporaryData["gender"].(string)
		height := state.TemporaryData["height"].(int)
		weight := state.TemporaryData["weight"].(int)
		goal := state.TemporaryData["goal"].(string)

		// Process goal text
		goalText := goal
		if goal == "Снизить вес" {
			goalText = "Снизить"
		} else if goal == "Поддерживать вес" {
			goalText = "Поддерживать"
		} else if goal == "Набрать вес" {
			goalText = "Набрать"
		}

		// Create user object
		user := &models.User{
			TelegramID: userID,
			ChatID:     chatID,
			Username:   message.From.UserName,
			Gender:     gender,
			Height:     height,
			Weight:     weight,
			Goal:       goalText,
		}

		// Save to database
		err := t.db.SaveUser(ctx, user)
		if err != nil {
			t.logger.Error("Failed to save user data", "error", err)
			msg := tgbotapi.NewMessage(chatID, "Извините, произошла ошибка при сохранении данных. Пожалуйста, попробуйте позже.")
			t.bot.Send(msg)
			return
		}

		// Move to payment state
		state.CurrentState = StatePayment

		// Send payment info
		msg := tgbotapi.NewMessage(chatID, "Спасибо! Ваши данные сохранены. Для получения персонализированного плана питания, требуется оплата в размере 1000 руб.")
		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		t.bot.Send(msg)

		// Create a Stripe checkout session
		// Create a Stripe checkout session
		successURL := fmt.Sprintf("https://t.me/%s?start=payment_success", t.bot.Self.UserName)
		cancelURL := fmt.Sprintf("https://t.me/%s?start=payment_cancel", t.bot.Self.UserName)

		sessionID, checkoutURL, err := t.stripeClient.CreateCheckoutSession(userID, successURL, cancelURL)
		if err != nil {
			t.logger.Error("Failed to create Stripe session", "error", err)
			msg := tgbotapi.NewMessage(chatID, "Извините, произошла ошибка при создании платежной сессии. Пожалуйста, попробуйте позже.")
			t.bot.Send(msg)
			return
		}

		// Save session ID to user state
		state.StripeSessionID = sessionID

		// Create a payment record in the database
		payment := &models.Payment{
			UserID:          user.ID,
			Amount:          1000,
			Currency:        "rub",
			StripePaymentID: sessionID,
			Status:          "pending",
		}
		err = t.db.SavePayment(ctx, payment)
		if err != nil {
			t.logger.Error("Failed to save payment record", "error", err)
			// Continue anyway, as this is not critical for the user
		}

		// Send the real payment link using URL directly from Stripe
		paymentMsg := tgbotapi.NewMessage(chatID, "Нажмите на кнопку ниже, чтобы перейти к оплате:")
		paymentMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Оплатить", checkoutURL),
			),
		)
		t.bot.Send(paymentMsg)
	default:
		// Unknown state, reset to start
		msg := tgbotapi.NewMessage(chatID, "Извините, произошла ошибка. Пожалуйста, используйте /start для начала заново.")
		t.bot.Send(msg)

		// Reset state
		t.stateMutex.Lock()
		t.userStates[userID] = &models.UserState{
			TelegramID:    userID,
			CurrentState:  StateStart,
			TemporaryData: make(map[string]interface{}),
		}
		t.stateMutex.Unlock()
	}
}

// handleCallbackQuery processes callback queries from inline keyboards
func (t *TelegramBot) handleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery) {
	// Process callback data
	t.logger.Info("Received callback query",
		"from", callbackQuery.From.UserName,
		"data", callbackQuery.Data)

	// Acknowledge the callback
	callback := tgbotapi.NewCallback(callbackQuery.ID, "")
	t.bot.Request(callback)

	// Handle specific callback actions if needed
}

// Stop gracefully shuts down the bot
func (t *TelegramBot) Stop(ctx context.Context) error {
	// Stop receiving updates
	t.bot.StopReceivingUpdates()

	// Allow time for handlers to complete
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(500 * time.Millisecond):
		return nil
	}
}

// handlePaymentSuccess processes successful payments
func (t *TelegramBot) handlePaymentSuccess(userID int64, paymentIntentID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.logger.Info("Processing successful payment", "userID", userID, "paymentID", paymentIntentID)

	// Get user from database
	user, err := t.db.GetUser(ctx, userID)
	if err != nil {
		t.logger.Error("Failed to get user data", "error", err, "userID", userID)
		return
	}

	// Update payment status
	err = t.db.UpdatePaymentStatus(ctx, paymentIntentID, "completed")
	if err != nil {
		t.logger.Error("Failed to update payment status", "error", err, "paymentID", paymentIntentID)
		return
	}

	// Get payment record
	payment, err := t.db.GetPaymentByStripeID(ctx, paymentIntentID)
	if err != nil {
		t.logger.Error("Failed to get payment record", "error", err, "paymentID", paymentIntentID)
		return
	}

	// Generate diet plan with GPT
	t.logger.Info("Generating diet plan with GPT", "userID", userID)
	planText, err := t.gptClient.GenerateDietPlan(ctx, user)
	if err != nil {
		t.logger.Error("Failed to generate diet plan", "error", err, "userID", userID)

		// Send error message to user
		msg := tgbotapi.NewMessage(user.ChatID, "К сожалению, произошла ошибка при создании плана питания. Пожалуйста, свяжитесь с поддержкой.")
		_, _ = t.bot.Send(msg)
		return
	}

	// Save diet plan to database
	dietPlan := &models.DietPlan{
		UserID:    user.ID,
		PaymentID: payment.ID,
		PlanText:  planText,
	}

	err = t.db.SaveDietPlan(ctx, dietPlan)
	if err != nil {
		t.logger.Error("Failed to save diet plan", "error", err, "userID", userID)
	}

	// Send diet plan to user
	t.logger.Info("Sending diet plan to user", "userID", userID, "chatID", user.ChatID)
	msg := tgbotapi.NewMessage(user.ChatID, "🎉 Ваш персонализированный план питания готов!\n\n"+planText)
	_, err = t.bot.Send(msg)
	if err != nil {
		t.logger.Error("Failed to send diet plan message", "error", err, "chatID", user.ChatID)
	}

	// Update user state
	t.stateMutex.Lock()
	if state, ok := t.userStates[userID]; ok {
		state.CurrentState = StateComplete
	}
	t.stateMutex.Unlock()
}
