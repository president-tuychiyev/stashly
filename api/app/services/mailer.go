package services

import (
	"errors"
	"strings"

	contractsmail "github.com/goravel/framework/contracts/mail"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	appmail "github.com/president-tuychiyev/stashly/api/app/mail"
)

// The framework only ships an SMTP transport, so the "log" mailer the contract
// asks for is implemented here: the same Mailer interface with a second
// implementation that writes the rendered message to the application log. That
// keeps local development and the test suite from needing an SMTP server, and
// it is the only place a one time code is ever written anywhere readable.

// Mailer delivers a rendered message to one address.
type Mailer interface {
	Send(to string, message appmail.Message) error
}

// ErrLogMailerNotAllowed is returned when a deployment that is not local or
// testing asks for the log mailer. Writing one time codes into the application
// log is a development convenience, never a delivery method.
var ErrLogMailerNotAllowed = errors.New(
	`MAIL_MAILER=log writes one time codes to the application log and is only allowed when APP_ENV is "local" or "testing"; set MAIL_MAILER=smtp and configure MAIL_HOST/MAIL_PORT`,
)

// NewMailer returns the mailer selected by MAIL_MAILER, which defaults to
// "smtp". The log mailer has to be asked for explicitly and is refused outside
// local and testing, so a production deployment cannot silently swallow every
// message it thinks it sent. An unknown value is an error rather than a
// fallback: guessing here is what hides a misconfiguration.
func NewMailer() (Mailer, error) {
	switch strings.ToLower(strings.TrimSpace(facades.Config().GetString("mail.mailer", "smtp"))) {
	case "", "smtp":
		return &SMTPMailer{}, nil
	case "log":
		if !AppIsLocalOrTesting() {
			return nil, ErrLogMailerNotAllowed
		}

		return &LogMailer{}, nil
	default:
		return nil, errors.New(`MAIL_MAILER must be one of "smtp" or "log"`)
	}
}

// AppIsLocalOrTesting reports whether the process runs in one of the two
// environments where development shortcuts (the log mailer, the default seed
// password) are acceptable.
func AppIsLocalOrTesting() bool {
	env := strings.ToLower(strings.TrimSpace(facades.Config().GetString("app.env")))

	return env == "local" || env == "testing"
}

// SMTPMailer sends through the goravel mail facade.
type SMTPMailer struct{}

func (r *SMTPMailer) Send(to string, message appmail.Message) error {
	host := facades.Config().GetString("mail.host")
	if host == "" {
		return errors.New("MAIL_MAILER is smtp but MAIL_HOST is empty")
	}

	// The framework derives the transport from the port alone (465 implicit
	// TLS, 587 STARTTLS, otherwise plain), so a configuration whose stated
	// encryption disagrees with its port would silently send in the clear.
	if err := checkEncryption(
		strings.ToLower(facades.Config().GetString("mail.encryption", "tls")),
		facades.Config().GetInt("mail.port"),
	); err != nil {
		return err
	}

	return facades.Mail().
		To([]string{to}).
		From(contractsmail.Address{
			Address: facades.Config().GetString("mail.from.address"),
			Name:    facades.Config().GetString("mail.from.name"),
		}).
		Subject(message.Subject).
		Content(contractsmail.Content{Text: message.Text, Html: message.HTML}).
		Send()
}

// checkEncryption refuses a port that cannot deliver the requested transport
// security.
func checkEncryption(encryption string, port int) error {
	switch encryption {
	case "ssl":
		if port != 465 {
			return errors.New("MAIL_ENCRYPTION=ssl requires MAIL_PORT=465")
		}
	case "tls", "":
		if port != 587 {
			return errors.New("MAIL_ENCRYPTION=tls requires MAIL_PORT=587")
		}
	case "none":
		if port == 465 || port == 587 {
			return errors.New("MAIL_ENCRYPTION=none cannot be used with MAIL_PORT 465 or 587")
		}
	default:
		return errors.New("MAIL_ENCRYPTION must be one of tls, ssl or none")
	}

	return nil
}

// LogMailer writes the message to the application log instead of delivering
// it. This is the local development and test transport.
type LogMailer struct{}

func (r *LogMailer) Send(to string, message appmail.Message) error {
	facades.Log().With(map[string]any{
		"mailer":  "log",
		"to":      to,
		"subject": message.Subject,
	}).Info("mail\nTo: " + to + "\nSubject: " + message.Subject + "\n\n" + message.Text)

	return nil
}
