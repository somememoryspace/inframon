package notifiers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"time"
)

const (
	StatusTooManyRequests = 429
)

type Message struct {
	Content string         `json:"content,omitempty"`
	Embeds  []DiscordEmbed `json:"embeds,omitempty"`
}

type DiscordEmbed struct {
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Color       int            `json:"color,omitempty"`
	Fields      []DiscordField `json:"fields,omitempty"`
}

type DiscordField struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

func SendToDiscordWebhook(discordWebhookDisable bool, webhookURL string, title string, description string, color int, address string, service string, networkZone string, instanceType string, retryCount int, rateLimitResetTime time.Duration, maxRetries int) error {
	if discordWebhookDisable {
		return fmt.Errorf("discord webhook notifications disabled")
	}
	now := time.Now()
	embed := DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Fields: []DiscordField{
			{Name: "Address", Value: address, Inline: true},
			{Name: "Service", Value: service, Inline: true},
			{Name: "Date", Value: now.Format("2006-01-02"), Inline: true},
			{Name: "Time", Value: now.Format("15:04:05"), Inline: true},
			{Name: "NetworkZone", Value: networkZone, Inline: true},
			{Name: "InstanceType", Value: instanceType, Inline: true},
		},
	}
	message := Message{Embeds: []DiscordEmbed{embed}}
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	return sendWithRetries(webhookURL, payload, retryCount, rateLimitResetTime, maxRetries)
}

func sendWithRetries(webhookURL string, payload []byte, retryCount int, rateLimitResetTime time.Duration, maxRetries int) error {
	for i := 0; i <= maxRetries; i++ {
		err := sendDiscordRequest(webhookURL, payload)
		if err == nil {
			return nil
		}
		if i < maxRetries {
			time.Sleep(rateLimitResetTime)
		}
	}
	return fmt.Errorf("failed to send request after %d retries", maxRetries)
}

func sendDiscordRequest(webhookURL string, payload []byte) error {
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == StatusTooManyRequests {
		return fmt.Errorf("rate limited, retrying")
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected response status: %s", resp.Status)
	}
	return nil
}

func SendSMTPMail(smtpDisable bool, smtpUsername string, smtpPassword string, smtpHost string, smtpTo string, smtpFrom string, smtpPort string, title string, description string, address string, service string, networkZone string, instanceType string) error {
	if smtpDisable {
		return fmt.Errorf("smtp push notifications disabled")
	}

	now := time.Now()
	body := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
		<style>
			body {
				font-family: Arial, sans-serif;
				line-height: 1.6;
				color: #333333;
				max-width: 600px;
				margin: 0 auto;
				padding: 20px;
			}
			.header {
				background-color: #f1f3f4;
				padding: 10px;
				border-radius: 5px;
				margin-bottom: 20px;
			}
			h2 {
				color: #1a73e8;
				background-color: #f1f3f4;
				margin-bottom: 10px;
				padding: 10px;
				border-radius: 5px;
				width: 300px
			}
			ul {
				list-style-type: none;
				padding: 0;
			}
			li {
				background-color: #f1f3f4;
				margin-bottom: 10px;
				padding: 10px;
				border-radius: 5px;
				width: 300px
			}
			strong {
				color: #1a73e8;
			}
		</style>
	</head>
	<body>
		<h2>%s</h2>
		<h2>%s</h2>
		<ul>
			<li><strong>Date:</strong> %s</li>
			<li><strong>Time:</strong> %s</li>
			<li><strong>Address:</strong> %s</li>
			<li><strong>Service:</strong> %s</li>
			<li><strong>NetworkZone:</strong> %s</li>
			<li><strong>InstanceType:</strong> %s</li>
		</ul>
	</body>
	</html>
	`,
		title,
		description,
		now.Format("2006-01-02"),
		now.Format("15:04:05"),
		address,
		service,
		networkZone,
		instanceType,
	)

	subject := title
	auth := smtp.PlainAuth("", smtpUsername, smtpPassword, smtpHost)
	to := []string{smtpTo}

	headers := make(map[string]string)
	headers["From"] = smtpFrom
	headers["To"] = smtpTo
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=\"utf-8\""

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	err := smtp.SendMail(addr, auth, smtpFrom, to, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	return nil
}
