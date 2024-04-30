package email

import (
	"crypto/tls"

	"dojo-api/utils"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	gomail "gopkg.in/mail.v2"
)

func SendEmail(to string, body string) error {
	err := godotenv.Load()
	if err != nil {
		log.Fatal().Msg("Error loading .env file")
	}

	password := utils.LoadDotEnv("EMAIL_PASSWORD")
	email_address := utils.LoadDotEnv("EMAIL_ADDRESS")

	m := gomail.NewMessage()

	// Set E-Mail sender
	m.SetHeader("From", email_address)

	// Set E-Mail receivers
	m.SetHeader("To", to)

	// Set E-Mail subject
	m.SetHeader("Subject", "API and Subscription key for Tensorplex Dojo subnet")

	// Set E-Mail body. You can set plain text or html with text/html
	m.SetBody("text/plain", body)

	// Settings for SMTP server
	d := gomail.NewDialer("smtp.gmail.com", 587, email_address, password)

	// This is only needed when SSL/TLS certificate is not valid on server.
	// In production this should be set to false.
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	// Now send E-Mail
	if err := d.DialAndSend(m); err != nil {
		log.Error().Err(err).Msg("Failed to send email")
		return err
	}

	return nil
}
