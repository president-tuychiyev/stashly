package feature

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

// otpCodeFor recovers the plain code of the outstanding one time code for an
// address and purpose.
//
// The table only stores a keyed digest, which is the point: nothing, not even
// the log, has to carry the code for the flow to work. A test still needs it,
// so the six digit space is searched against the stored digest with the same
// function the service uses. No production code has to grow a back door for
// the sake of the suite.
//
// services.OtpDigester binds the HMAC to the address and purpose once and
// resets it per candidate, so the search allocates one hash instead of a
// million: that is the difference between a fraction of a second and several.
func otpCodeFor(t *testing.T, email, purpose string) string {
	t.Helper()

	var record models.OtpCode
	err := facades.Orm().Query().Model(&models.OtpCode{}).
		Where("email", services.NormalizeEmail(email)).
		Where("purpose", purpose).
		WhereNull("consumed_at").
		OrderByDesc("id").First(&record)
	require.NoError(t, err, "no outstanding %s code for %s", purpose, email)
	require.NotZero(t, record.ID, "no outstanding %s code for %s", purpose, email)

	return crackOtpDigest(t, record.Email, record.Purpose, record.CodeHash)
}

// crackOtpDigest searches the six digit space for the code behind one digest.
func crackOtpDigest(t *testing.T, email, purpose, digest string) string {
	t.Helper()

	digestOf := services.OtpDigester(email, purpose)
	for candidate := 0; candidate < 1000000; candidate++ {
		code := fmt.Sprintf("%06d", candidate)
		if digestOf(code) == digest {
			return code
		}
	}

	t.Fatalf("the stored digest matches no six digit code")

	return ""
}

// forgetOtpCodes drops every code issued for an address, so a test that sends
// twice is not stopped by the resend gap of the one before it.
func forgetOtpCodes(email string) {
	_, _ = facades.Orm().Query().Model(&models.OtpCode{}).
		Where("email", services.NormalizeEmail(email)).Delete()
}
