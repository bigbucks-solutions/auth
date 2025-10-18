package emailservice

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"strings"
	"sync"

	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/settings"
)

var (
	// singleton instance
	instance EmailSender
	once     sync.Once
)

// EmailSender interface defines the contract for sending emails
type EmailSender interface {
	SendEmail(toEmail string, templatePath string, params any) error
}

type EmailService struct {
	config *settings.Settings
}

var _ EmailSender = (*EmailService)(nil)

func NewEmailService(config *settings.Settings) EmailSender {
	return &EmailService{
		config: config,
	}
}

// SendEmail sends an email using the specified template and parameters
func (e *EmailService) SendEmail(toEmail string, templatePath string, params any) error {
	// Parse and execute template
	body, err := e.parseTemplate(templatePath, params)
	if err != nil {
		loging.Logger.Errorf("Failed to parse email template %s: %v", templatePath, err)
		return fmt.Errorf("failed to parse email template: %w", err)
	}

	// Get subject from params or use default
	subject := "Notification"

	return e.sendEmail(toEmail, subject, body)
}

// parseTemplate parses the HTML template and replaces placeholders with provided parameters
func (e *EmailService) parseTemplate(templatePath string, params any) (string, error) {
	// Check if template file exists
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return "", fmt.Errorf("template file does not exist: %s", templatePath)
	}

	// Parse the template file
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse template file: %w", err)
	}

	// Execute template with parameters
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// sendEmail sends the actual email via SMTP
func (e *EmailService) sendEmail(toEmail string, subject string, body string) error {
	// Validate email configuration
	if e.config.SMTPHost == "" || e.config.SMTPPort == "" {
		return fmt.Errorf("SMTP host and port are required")
	}

	// Create SMTP auth if username and password are provided
	var auth smtp.Auth
	if e.config.SMTPUsername != "" && e.config.SMTPPassword != "" {
		auth = smtp.PlainAuth("", e.config.SMTPUsername, e.config.SMTPPassword, e.config.SMTPHost)
	}

	// Compose email message
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\n", e.config.SMTPFrom, toEmail, subject)

	// Determine content type based on body content
	if strings.Contains(body, "<html>") || strings.Contains(body, "<HTML>") {
		msg += "Content-Type: text/html; charset=UTF-8\r\n"
	} else {
		msg += "Content-Type: text/plain; charset=UTF-8\r\n"
	}

	msg += "\r\n" + body

	// Send email
	addr := e.config.SMTPHost + ":" + e.config.SMTPPort
	err := smtp.SendMail(addr, auth, e.config.SMTPFrom, []string{toEmail}, []byte(msg))
	if err != nil {
		loging.Logger.Errorf("Failed to send email to %s: %v", toEmail, err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	loging.Logger.Infof("Email sent successfully to %s with subject: %s", toEmail, subject)
	return nil
}

// GetInstance returns the singleton instance of EmailService
func GetInstance() EmailSender {
	once.Do(func() {
		instance = &EmailService{
			config: settings.Current,
		}
	})
	return instance
}

// SendEmail is a package-level function that uses the singleton instance
func SendEmail(toEmail string, templatePath string, params any) error {
	return GetInstance().SendEmail(toEmail, templatePath, params)
}
