package notifiers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
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

type InstanceStatus struct {
	Address      string
	Service      string
	NetworkZone  string
	InstanceType string
	Protocol     string
	Status       bool
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

func SendToDiscordWebhookSystem(discordWebhookDisable bool, webhookURL string, title string, description string, color int, retryCount int, rateLimitResetTime time.Duration, maxRetries int) error {
	if discordWebhookDisable {
		return fmt.Errorf("discord webhook notifications disabled")
	}
	now := time.Now()
	embed := DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Fields: []DiscordField{
			{Name: "Date", Value: now.Format("2006-01-02"), Inline: true},
			{Name: "Time", Value: now.Format("15:04:05"), Inline: true},
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

func SendStatusSummaryToDiscord(healthCronWebhookDisable bool, discordWebhookDisable bool, webhookURL string, icmpStatuses []InstanceStatus, httpStatuses []InstanceStatus, retryCount int, rateLimitResetTime time.Duration, maxRetries int) error {
	if discordWebhookDisable {
		return fmt.Errorf("discord webhook notifications disabled")
	}
	if healthCronWebhookDisable {
		return fmt.Errorf("healthConWebhookDisable webhook notifications disabled")
	}
	var failedServices []string
	for _, status := range icmpStatuses {
		if !status.Status {
			failedServices = append(failedServices, fmt.Sprintf("ICMP: %s (%s)", status.Address, status.Service))
		}
	}
	for _, status := range httpStatuses {
		if !status.Status {
			failedServices = append(failedServices, fmt.Sprintf("HTTP: %s (%s)", status.Address, status.Service))
		}
	}
	now := time.Now()

	var embed DiscordEmbed
	if len(failedServices) == 0 {
		embed = DiscordEmbed{
			Title: "Scheduled Report",
			Color: 0x00FF00,
			Fields: []DiscordField{
				{
					Name:   "Status",
					Value:  "All Pass",
					Inline: false,
				},
				{Name: "Date", Value: now.Format("2006-01-02"), Inline: true},
				{Name: "Time", Value: now.Format("15:04:05"), Inline: true},
			},
		}
	} else {
		failedServicesStr := strings.Join(failedServices, "\n")
		embed = DiscordEmbed{
			Title: "Scheduled Report",
			Color: 0xFF0000,
			Fields: []DiscordField{
				{
					Name:   "Failing Services",
					Value:  failedServicesStr,
					Inline: false,
				},
				{Name: "Date", Value: now.Format("2006-01-02"), Inline: true},
				{Name: "Time", Value: now.Format("15:04:05"), Inline: true},
			},
		}
	}

	message := Message{Embeds: []DiscordEmbed{embed}}
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return sendWithRetries(webhookURL, payload, retryCount, rateLimitResetTime, maxRetries)
}

func SendSMTPMail(smtpDisable bool, smtpUsername string, smtpPassword string, smtpHost string, smtpTo string, smtpFrom string, smtpPort string, title string, description string, address string, service string, networkZone string, instanceType string) error {
	if smtpDisable {
		return fmt.Errorf("smtp push notifications disabled")
	}

	now := time.Now()
	body := fmt.Sprintf(`
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Inframon Notification</title>
		<style>
			body {
				font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
				line-height: 1.6;
				color: black; /* Force default color to black */
				max-width: 600px;
				margin: 0 auto;
				padding: 20px;
				background-color: #f2f2f2;
				justify-content: center;
				display: grid;
			}
			.container {
				background-color: #ffffff;
				border-radius: 8px;
				box-shadow: 0 2px 4px rgba(0,0,0,0.3);
				padding: 10px;
			}
			.header {
				background-color: #4682B4;
				color: #ffffff;
				padding: 15px;
				border-radius: 8px 8px 0 0;
				margin: -10px -10px 10px -10px;
			}
			h2 {
				margin: 0;
				font-size: 17px;
				color: white; /* Ensure header text is white */
			}
			ul {
				list-style-type: none;
				padding: 0;
				font-size: 12px;
				color: black; /* Ensure list text is black */
			}
			li {
				background-color: #f2f2f2;
				color: black; /* Ensure list item text is black */
				margin-bottom: 10px;
				padding: 15px;
				border-radius: 5px;
				border-left: 5px solid #4682B4;
				transition: all 0.3s ease;
			}
			strong {
				color: #4682B4;
				font-weight: 600;
			}
			.footer {
				text-align: center;
				margin-top: 20px;
				font-size: 14px;
				color: #888888;
			}
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h2>Inframon Notification</h2>
			</div>
			<ul>
			<li><strong>Status:</strong> %s</li>
			<li><strong>Notification:</strong> %s</li>
			<li><strong>Date:</strong> %s</li>
			<li><strong>Time:</strong> %s</li>
			<li><strong>Address:</strong> %s</li>
			<li><strong>Service:</strong> %s</li>
			<li><strong>NetworkZone:</strong> %s</li>
			<li><strong>InstanceType:</strong> %s</li>
			</ul>
			<div class="footer">
				This is an automated notification. Please do not reply.
			</div>
		</div>
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

	subject := fmt.Sprintf("Inframon: %s :: %s :: %s", title, description, service)
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

func SendSMTPMailSystem(smtpDisable bool, smtpUsername string, smtpPassword string, smtpHost string, smtpTo string, smtpFrom string, smtpPort string, title string, description string) error {
	if smtpDisable {
		return fmt.Errorf("smtp push notifications disabled")
	}

	now := time.Now()
	body := fmt.Sprintf(`
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Inframon Notification</title>
		<style>
			body {
				font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
				line-height: 1.6;
				color: black; /* Force default color to black */
				max-width: 600px;
				margin: 0 auto;
				padding: 20px;
				background-color: #f2f2f2;
				justify-content: center;
				display: grid;
			}
			.container {
				background-color: #ffffff;
				border-radius: 8px;
				box-shadow: 0 2px 4px rgba(0,0,0,0.3);
				padding: 10px;
			}
			.header {
				background-color: #4682B4;
				color: #ffffff;
				padding: 15px;
				border-radius: 8px 8px 0 0;
				margin: -10px -10px 10px -10px;
			}
			h2 {
				margin: 0;
				font-size: 17px;
				color: white; /* Ensure header text is white */
			}
			ul {
				list-style-type: none;
				padding: 0;
				font-size: 12px;
				color: black; /* Ensure list text is black */
			}
			li {
				background-color: #f2f2f2;
				color: black; /* Ensure list item text is black */
				margin-bottom: 10px;
				padding: 15px;
				border-radius: 5px;
				border-left: 5px solid #4682B4;
				transition: all 0.3s ease;
			}
			strong {
				color: #4682B4;
				font-weight: 600;
			}
			.footer {
				text-align: center;
				margin-top: 20px;
				font-size: 14px;
				color: #888888;
			}
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h2>Inframon Notification</h2>
			</div>
			<ul>
				<li><strong>Status:</strong> %s</li>
				<li><strong>Description:</strong> %s</li>
				<li><strong>Date:</strong> %s</li>
				<li><strong>Time:</strong> %s</li>
			</ul>
			<div class="footer">
				This is an automated notification. Please do not reply.
			</div>
		</div>
	</body>
	</html>	
	`,
		title,
		description,
		now.Format("2006-01-02"),
		now.Format("15:04:05"),
	)

	subject := fmt.Sprintf("Inframon: %s :: %s", title, description)
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

func SendStatusSummaryToSMTP(smtpDisable bool, healthCronSmtpDisable bool, smtpUsername string, smtpPassword string, smtpHost string, smtpTo string, smtpFrom string, smtpPort string, icmpStatuses []InstanceStatus, httpStatuses []InstanceStatus) error {
	if smtpDisable {
		return fmt.Errorf("smtp notifications disabled")
	}
	if healthCronSmtpDisable {
		return fmt.Errorf("health cron smtp notifications disabled")
	}

	var failedServices []string
	for _, status := range icmpStatuses {
		if !status.Status {
			failedServices = append(failedServices, fmt.Sprintf("ICMP: %s (%s)", status.Address, status.Service))
		}
	}
	for _, status := range httpStatuses {
		if !status.Status {
			failedServices = append(failedServices, fmt.Sprintf("HTTP: %s (%s)", status.Address, status.Service))
		}
	}

	now := time.Now()
	subject := "Inframon: Scheduled Report"
	var status, statusColor string

	if len(failedServices) == 0 {
		status = "All Pass"
		statusColor = "#00a600"
	} else {
		status = "Failing Services"
		statusColor = "#b70000"
	}

	body := fmt.Sprintf(`
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Inframon Scheduled Report</title>
		<style>
			body {
				font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
				line-height: 1.6;
				color: black; /* Force default color to black */
				max-width: 600px;
				margin: 0 auto;
				padding: 20px;
				background-color: #f2f2f2;
			}
			.container {
				background-color: #ffffff;
				border-radius: 8px;
				box-shadow: 0 2px 4px rgba(0,0,0,0.1);
				padding: 20px;
			}
			.header {
				background-color: #4682B4;
				color: white; /* Ensure header text is white */
				padding: 15px;
				border-radius: 8px 8px 0 0;
				margin: -20px -20px 20px -20px;
			}
			h2 {
				margin: 0;
				font-size: 24px;
				color: white; /* Force header text to be white */
			}
			.h3-failing-services {
				background-color: #4682B4;
				color: white; /* Ensure header text is white */
				padding: 10px;
				border-radius: 6px 6px 0 0;
				margin: 20px 0 10px 0;
				font-size: 20px;
			}
			.status-label {
				font-size: 14px;
				font-weight: bold;
				color: black; /* Force default color to black */
			}
			.status-text {
				font-size: 14px;
				font-weight: bold;
				color: %s; /* Apply status color here */
			}
			ul {
				list-style-type: none;
				padding: 0;
			}
			li {
				font-size: 14px;
				background-color: #f8f8f8;
				margin-bottom: 10px;
				padding: 10px;
				border-radius: 5px;
				color: black; /* Ensure list item text is black */
			}
			.footer {
				text-align: center;
				margin-top: 20px;
				font-size: 14px;
				color: #888888; /* Keep footer in grey */
			}
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h2>Inframon Scheduled Report</h2>
			</div>
			<ul>
				<li>
					<div class="status-label">Status: <span class="status-text">%s</span></div>
				</li>
				<li><strong>Date:</strong> <span style="color: black;">%s</span></li>
				<li><strong>Time:</strong> <span style="color: black;">%s</span></li>
			</ul>
			%s
			<div class="footer">
				This is an automated notification. Please do not reply.
			</div>
		</div>
	</body>
	</html>
	`,
		statusColor,
		status,
		now.Format("2006-01-02"),
		now.Format("15:04:05"),
		func() string {
			if len(failedServices) > 0 {
				return `<h3 class="h3-failing-services">Failing Services:</h3><ul><li>` + strings.Join(failedServices, "</li><li>") + `</li></ul>`
			}
			return ""
		}(),
	)

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
