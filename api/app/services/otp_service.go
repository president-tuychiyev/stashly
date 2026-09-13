package services

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/support/carbon"
	"github.com/spf13/cast"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	appmail "github.com/president-tuychiyev/stashly/api/app/mail"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

const (
	// OtpTTL is how long an issued code stays usable.
	OtpTTL = 10 * time.Minute
	// OtpResendGap is the quiet period between two codes for the same address
	// and purpose.
	OtpResendGap = 60 * time.Second
	// OtpMaxAttempts is how many wrong codes invalidate the issued one.
	OtpMaxAttempts = 5
)

// ErrOtpResendTooSoon is returned by Issue while the resend gap is still
// running. The HTTP layer turns it into a 429.
var ErrOtpResendTooSoon = errors.New("a code was already sent, try again in a minute")

// ErrOtpInvalid covers every reason a code is refused that the caller must not
// be able to tell apart: no code issued, wrong code, already used, or too many
// wrong attempts.
var ErrOtpInvalid = errors.New("the code is invalid or has expired")

// ErrOtpMailFailed wraps whatever the mailer reported. The controllers answer
// a generic 500 and log this, so an SMTP hostname or credential never reaches
// the caller.
var ErrOtpMailFailed = errors.New("could not send email")

// OtpIssueInput describes one code to issue.
type OtpIssueInput struct {
	// Email is where the code is sent and half of the key it is stored under.
	Email string
	// Purpose is one of the models.OtpPurpose* constants.
	Purpose string
	// Name is the recipient's display name, used in the message body only.
	Name string
	// NewEmail is the address an email change moves the account to. It is only
	// read for the email_change purpose, where Email is already the new one.
	NewEmail string
	// UserID is the account the code belongs to. It is stored in its own
	// indexed column so a code can be looked up by user without scanning the
	// meta, and so every outstanding code of the same purpose for that user is
	// invalidated when a new one is issued, whatever address it was sent to.
	UserID *uint
	// Meta is stored with the code and handed back by Verify, e.g. the address
	// an email change moves the account to.
	Meta map[string]any
}

type OtpService struct {
	mailer Mailer
}

// NewOtpService builds the service with the configured mailer. A mail
// configuration the service cannot honour is fatal here rather than at send
// time: the controllers are constructed while the routes are registered, so an
// unusable MAIL_MAILER stops the boot instead of letting every OTP endpoint
// answer 204 without sending anything.
func NewOtpService() *OtpService {
	mailer, err := NewMailer()
	if err != nil {
		facades.Log().Fatal("mail is misconfigured: " + err.Error())
	}

	return &OtpService{mailer: mailer}
}

// Issue invalidates whatever code is outstanding for the address and purpose,
// stores a fresh one and mails it out.
//
// The gap check, the invalidation and the insert share one transaction, and
// the partial unique index on (email, purpose) WHERE consumed_at IS NULL makes
// a second concurrent issue fail rather than leave two usable codes behind.
func (r *OtpService) Issue(input OtpIssueInput) error {
	var (
		record *models.OtpCode
		code   string
	)

	err := facades.Orm().Transaction(func(tx orm.Query) error {
		var err error
		record, code, err = r.store(tx, input)

		return err
	})
	if err != nil {
		return err
	}

	if err := r.deliver(input, record.Email, code); err != nil {
		// The row would otherwise hold the resend gap shut for a code nobody
		// ever received.
		if _, deleteErr := facades.Orm().Query().Model(&models.OtpCode{}).
			Where("id", record.ID).Delete(); deleteErr != nil {
			facades.Log().Warning("could not drop an undelivered otp code: " + deleteErr.Error())
		}

		return err
	}

	return nil
}

// IssueWithin is Issue on a transaction the caller owns, for the flows that
// have to create the account and its first code together. The mail is sent
// before the caller commits, so a delivery failure rolls the whole thing back
// instead of leaving an account nobody can activate.
func (r *OtpService) IssueWithin(tx orm.Query, input OtpIssueInput) error {
	record, code, err := r.store(tx, input)
	if err != nil {
		return err
	}

	return r.deliver(input, record.Email, code)
}

// store runs the database half of an issue on the given query handle.
func (r *OtpService) store(tx orm.Query, input OtpIssueInput) (*models.OtpCode, string, error) {
	email := NormalizeEmail(input.Email)
	if email == "" {
		return nil, "", errors.New("email is required")
	}

	tooSoon, err := tx.Model(&models.OtpCode{}).
		Where("email", email).Where("purpose", input.Purpose).
		Where("created_at > ?", carbon.Now().SubSeconds(int(OtpResendGap.Seconds())).StdTime()).
		Exists()
	if err != nil {
		return nil, "", err
	}
	if tooSoon {
		return nil, "", ErrOtpResendTooSoon
	}

	now := carbon.Now().StdTime()
	if _, err := tx.Model(&models.OtpCode{}).
		Where("email", email).Where("purpose", input.Purpose).WhereNull("consumed_at").
		Update("consumed_at", now); err != nil {
		return nil, "", err
	}

	// An email change is stored under the new address, so the same user can
	// have outstanding codes under several addresses. Only the newest may be
	// usable, otherwise an abandoned request stays a live path to an address
	// the user has since thought better of.
	if input.UserID != nil {
		if _, err := tx.Model(&models.OtpCode{}).
			Where("user_id", *input.UserID).Where("purpose", input.Purpose).WhereNull("consumed_at").
			Update("consumed_at", now); err != nil {
			return nil, "", err
		}
	}

	code, err := generateOtpCode()
	if err != nil {
		return nil, "", err
	}

	meta := input.Meta
	if meta == nil {
		meta = map[string]any{}
	}

	record := &models.OtpCode{
		Email:     email,
		Purpose:   input.Purpose,
		UserID:    input.UserID,
		CodeHash:  OtpDigest(email, input.Purpose, code),
		ExpiresAt: carbon.NewDateTime(carbon.Now().AddSeconds(int(OtpTTL.Seconds()))),
		Attempts:  0,
		Meta:      meta,
	}
	if err := tx.Create(record); err != nil {
		// Two callers raced through the gap check; the partial unique index
		// caught the second one, which is exactly the "already sent" case.
		if isUniqueViolation(err) {
			return nil, "", ErrOtpResendTooSoon
		}

		return nil, "", err
	}

	return record, code, nil
}

// deliver renders and sends the message, wrapping every failure so the caller
// can answer a generic 500 while the detail goes to the log.
func (r *OtpService) deliver(input OtpIssueInput, email, code string) error {
	message, err := r.render(input, code)
	if err != nil {
		return fmt.Errorf("%w: rendering the %s message failed: %w", ErrOtpMailFailed, input.Purpose, err)
	}

	if err := r.mailer.Send(email, message); err != nil {
		return fmt.Errorf("%w: sending the %s code failed: %w", ErrOtpMailFailed, input.Purpose, err)
	}

	return nil
}

// Verify checks a presented code and, when it matches, consumes it and returns
// the meta it was issued with.
//
// The attempt is counted before the comparison and by the database itself, so
// a burst of parallel guesses cannot each read the same counter and write it
// back: after OtpMaxAttempts the code is burnt no matter how the guesses were
// interleaved.
func (r *OtpService) Verify(email, purpose, code string) (map[string]any, error) {
	email = NormalizeEmail(email)

	var record models.OtpCode
	err := facades.Orm().Query().Model(&models.OtpCode{}).
		Where("email", email).Where("purpose", purpose).WhereNull("consumed_at").
		OrderByDesc("id").First(&record)
	if err != nil || record.ID == 0 {
		return nil, ErrOtpInvalid
	}

	attempts, err := r.registerAttempt(record.ID)
	if err != nil {
		// Either the row was consumed between the read and the update, or the
		// database is unhappy; both are "this code does not work".
		return nil, ErrOtpInvalid
	}
	if attempts > OtpMaxAttempts {
		_, _ = r.consume(record.ID)

		return nil, ErrOtpInvalid
	}

	if record.ExpiresAt == nil || !record.ExpiresAt.StdTime().After(time.Now()) {
		_, _ = r.consume(record.ID)

		return nil, ErrOtpInvalid
	}

	expected := OtpDigest(email, purpose, strings.TrimSpace(code))
	if !hmac.Equal([]byte(record.CodeHash), []byte(expected)) {
		if attempts >= OtpMaxAttempts {
			// The code is burnt once it has been guessed at too often; a new
			// one has to be requested.
			_, _ = r.consume(record.ID)
		}

		return nil, ErrOtpInvalid
	}

	// Consuming is conditional on the row still being unconsumed, so two
	// requests presenting the same correct code cannot both succeed.
	consumed, err := r.consume(record.ID)
	if err != nil || !consumed {
		return nil, ErrOtpInvalid
	}

	meta := record.Meta
	if meta == nil {
		meta = map[string]any{}
	}
	if record.UserID != nil {
		meta["user_id"] = *record.UserID
	}

	return meta, nil
}

// registerAttempt increments the attempt counter of an unconsumed code and
// returns the new value. The increment and the read share a transaction, so
// the row lock the update takes serialises concurrent verifications.
func (r *OtpService) registerAttempt(id uint) (int16, error) {
	var attempts int16

	err := facades.Orm().Transaction(func(tx orm.Query) error {
		result, err := tx.Exec(
			`UPDATE otp_codes SET attempts = attempts + 1 WHERE id = ? AND consumed_at IS NULL`, id)
		if err != nil {
			return err
		}
		if result.RowsAffected == 0 {
			return ErrOtpInvalid
		}

		var record models.OtpCode
		if err := tx.Model(&models.OtpCode{}).Where("id", id).First(&record); err != nil {
			return err
		}
		attempts = record.Attempts

		return nil
	})

	return attempts, err
}

// consume marks a code used. It reports false when the row was already
// consumed, which is what makes a replay of a correct code fail.
func (r *OtpService) consume(id uint) (bool, error) {
	result, err := facades.Orm().Query().Model(&models.OtpCode{}).
		Where("id", id).WhereNull("consumed_at").
		Update("consumed_at", carbon.Now().StdTime())
	if err != nil {
		return false, err
	}

	return result.RowsAffected > 0, nil
}

func (r *OtpService) render(input OtpIssueInput, code string) (appmail.Message, error) {
	data := appmail.Data{
		Name:     input.Name,
		Code:     code,
		Minutes:  int(OtpTTL.Minutes()),
		NewEmail: input.NewEmail,
		AppName:  facades.Config().GetString("app.name", "Stashly"),
	}

	switch input.Purpose {
	case models.OtpPurposePasswordReset:
		return appmail.PasswordReset(data)
	case models.OtpPurposeEmailChange:
		if data.NewEmail == "" {
			data.NewEmail = NormalizeEmail(input.Email)
		}

		return appmail.EmailChange(data)
	default:
		return appmail.Verification(data)
	}
}

// isUniqueViolation reports whether the driver refused a write because of a
// unique index. Postgres reports 23505; the message is matched as well so a
// driver that only surfaces the text is still recognised.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	text := err.Error()

	return strings.Contains(text, "23505") ||
		strings.Contains(strings.ToLower(text), "duplicate key value")
}

// otpKeyOnce guards the derived key so the HMAC of the application key is
// computed once per process.
var (
	otpKeyOnce  sync.Once
	otpKeyValue []byte
)

// OtpKey is the key the code digests are computed under. It is derived from
// the application key rather than being it, so the one secret is never used
// raw in two places: HMAC-SHA256(app.key, "otp").
func OtpKey() []byte {
	otpKeyOnce.Do(func() {
		mac := hmac.New(sha256.New, []byte(facades.Config().GetString("app.key")))
		mac.Write([]byte("otp"))
		otpKeyValue = mac.Sum(nil)
	})

	return otpKeyValue
}

// OtpDigester returns a digest function bound to one address and purpose. The
// HMAC state is allocated once and reset per code, which is what makes the
// test suite's search over the six digit space cheap. The returned function is
// not safe for concurrent use.
func OtpDigester(email, purpose string) func(code string) string {
	mac := hmac.New(sha256.New, OtpKey())
	prefix := []byte(NormalizeEmail(email) + "\x00" + purpose + "\x00")
	sum := make([]byte, 0, sha256.Size)

	return func(code string) string {
		mac.Reset()
		mac.Write(prefix)
		mac.Write([]byte(code))

		return hex.EncodeToString(mac.Sum(sum[:0]))
	}
}

// OtpDigest is the keyed digest a code is stored as. The address and the
// purpose are mixed in, so a digest cannot be replayed against another flow
// even if the table leaks, and the derived key means the digest cannot be
// recomputed without the application key.
func OtpDigest(email, purpose, code string) string {
	return OtpDigester(email, purpose)(code)
}

// generateOtpCode returns six uniformly random digits.
func generateOtpCode() (string, error) {
	pick, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", pick.Int64()), nil
}

// NormalizeEmail trims and lowercases an address so that it is stored and
// looked up the same way everywhere.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// PendingEmailChange returns the address an outstanding email change would
// move a user to. The code is stored under the new address, but the confirm
// endpoint only knows the authenticated user, so the pending record is found
// through the indexed user_id column the code was issued with.
func (r *OtpService) PendingEmailChange(userID uint) (string, bool) {
	var record models.OtpCode
	err := facades.Orm().Query().Model(&models.OtpCode{}).
		Where("purpose", models.OtpPurposeEmailChange).
		Where("user_id", userID).
		WhereNull("consumed_at").
		Where("expires_at > ?", carbon.Now().StdTime()).
		OrderByDesc("id").First(&record)
	if err != nil || record.ID == 0 {
		return "", false
	}

	return record.Email, true
}

// MetaUserID reads the user id out of the meta a code was issued with. JSON
// numbers come back as float64, but an int is accepted too.
func MetaUserID(meta map[string]any) uint {
	value, ok := meta["user_id"]
	if !ok {
		return 0
	}

	return cast.ToUint(value)
}
