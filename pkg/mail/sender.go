package mail

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"os"
	"strconv"
)

// SMTPConfig holds SMTP connection configuration.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	TLSMode  string // "none", "starttls", "tls" (implicit TLS)
}

// NewSMTPConfigFromEnv creates an SMTPConfig from environment variables.
func NewSMTPConfigFromEnv() *SMTPConfig {
	port, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))

	// Determine TLS mode: SMTP_TLS_MODE takes priority, fall back to SMTP_USE_TLS for compat
	tlsMode := getEnv("SMTP_TLS_MODE", "")
	if tlsMode == "" {
		useTLS, _ := strconv.ParseBool(getEnv("SMTP_USE_TLS", "true"))
		if !useTLS {
			tlsMode = "none"
		} else if port == 465 {
			tlsMode = "tls"
		} else {
			tlsMode = "starttls"
		}
	}

	return &SMTPConfig{
		Host:     getEnv("SMTP_HOST", "localhost"),
		Port:     port,
		Username: getEnv("SMTP_USERNAME", ""),
		Password: getEnv("SMTP_PASSWORD", ""),
		From:     getEnv("SMTP_FROM", "noreply@example.com"),
		TLSMode:  tlsMode,
	}
}

// Sender sends emails via SMTP.
type Sender struct {
	config *SMTPConfig
}

// NewSender creates a new email sender.
func NewSender(config *SMTPConfig) *Sender {
	return &Sender{config: config}
}

// Send sends an email with HTML body.
func (s *Sender) Send(to, subject, htmlBody string) error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	headers := map[string]string{
		"From":         s.config.From,
		"To":           to,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/html; charset=UTF-8",
	}

	var message string
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + htmlBody

	var auth smtp.Auth
	if s.config.Username != "" {
		auth = smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
	}

	switch s.config.TLSMode {
	case "tls":
		return s.sendWithImplicitTLS(addr, auth, to, []byte(message))
	case "starttls":
		return s.sendWithSTARTTLS(addr, auth, to, []byte(message))
	default:
		return smtp.SendMail(addr, auth, s.config.From, []string{to}, []byte(message))
	}
}

func (s *Sender) sendWithImplicitTLS(addr string, auth smtp.Auth, to string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: s.config.Host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}

	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	return s.sendViaSMTPClient(client, auth, to, msg)
}

func (s *Sender) sendWithSTARTTLS(addr string, auth smtp.Auth, to string, msg []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()

	tlsConfig := &tls.Config{
		ServerName: s.config.Host,
		MinVersion: tls.VersionTLS12,
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("STARTTLS failed: %w", err)
	}

	return s.sendViaSMTPClient(client, auth, to, msg)
}

func (s *Sender) sendViaSMTPClient(client *smtp.Client, auth smtp.Auth, to string, msg []byte) error {
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err := client.Mail(s.config.From); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("SMTP RCPT TO failed: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("SMTP write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("SMTP close failed: %w", err)
	}

	return client.Quit()
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
