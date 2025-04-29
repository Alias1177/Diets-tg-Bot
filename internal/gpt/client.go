// internal/gpt/client.go
package gpt

import (
	"awesomeProject/Diets_Bot/internal/models"
	"context"
	"fmt"
)

type Client struct {
	client *openai.Client
	model  string
}

func NewClient(apiKey string) *Client {
	return &Client{
		client: openai.NewClient(apiKey),
		model:  "gpt-4",
	}
}

func (c *Client) WithModel(model string) *Client {
	c.model = model
	return c
}

func (c *Client) GenerateDietPlan(ctx context.Context, user *models.User) (string, error) {
	// Подготовка запроса для GPT
	gender := user.Gender
	height := user.Height
	weight := user.Weight
	goal := user.Goal

	prompt := fmt.Sprintf(
		"Создай персонализированный план питания для человека со следующими параметрами:\n"+
			"- Пол: %s\n"+
			"- Рост: %d см\n"+
			"- Вес: %d кг\n"+
			"- Цель: %s вес\n\n"+
			"План должен включать:\n"+
			"1. Общее количество калорий в день\n"+
			"2. Распределение белков, жиров и углеводов\n"+
			"3. Пример меню на 7 дней с указанием времени приема пищи\n"+
			"4. Рекомендации по питьевому режиму\n"+
			"5. Дополнительные рекомендации для достижения цели\n",
		gender, height, weight, goal,
	)

	// Создание запроса к OpenAI
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "Ты опытный диетолог. Твоя задача создать персонализированный план питания на основе параметров пользователя.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		MaxTokens:   2500,
		Temperature: 0.7,
	}

	// Вызов API OpenAI
	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from GPT API")
	}

	return resp.Choices[0].Message.Content, nil
}
