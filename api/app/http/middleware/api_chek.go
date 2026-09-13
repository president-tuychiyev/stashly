package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/support/carbon"
	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/models"
	"github.com/president-tuychiyev/stashly/api/app/services"
)

type apiCheck struct{}

func ApiCheck() http.Middleware {
	return &apiCheck{}
}

func (r *apiCheck) Signature() string {
	return "api_check"
}

func (r *apiCheck) Handle(ctx http.Context) {
	username := ctx.Request().Headers().Get("x-username")
	password := ctx.Request().Headers().Get("x-password")
	language := ctx.Request().Headers().Get("x-language")
	device := ctx.Request().Headers().Get("x-device")

	if username == "" || password == "" || language == "" || device == "" {
		abort(ctx, http.StatusUnauthorized, "X-Username, X-Password, X-Language and X-Device headers are required")
		return
	}

	if language != "uz" && language != "ru" && language != "en" {
		abort(ctx, http.StatusUnauthorized, "X-Language must be one of 'uz', 'ru', or 'en'")
		return
	}

	cacheKey := services.ClientCacheKey(username)
	client, ok := facades.Cache().Get(cacheKey).(models.Client)
	if !ok || client.ID == 0 {
		client = models.Client{}
		if err := facades.Orm().Query().Where("username", username).FirstOrFail(&client); err != nil {
			abort(ctx, http.StatusUnauthorized, "X-Client not found or invalid")
			return
		}

		facades.Cache().Put(cacheKey, client, 60*time.Minute)
	}

	if !checkClientPassword(username, password, client.Password) {
		abort(ctx, http.StatusUnauthorized, "X-Password is incorrect")
		return
	}

	if client.Status != "active" {
		abort(ctx, http.StatusForbidden, "Client is "+client.Status)
		return
	}

	deviceModel, err := touchDevice(ctx, client.ID, device)
	if err != nil {
		abort(ctx, http.StatusInternalServerError, "Device could not be registered")
		return
	}

	ctx.WithValue("client_id", client.ID)
	ctx.WithValue("device_id", deviceModel.ID)
	ctx.WithValue("device_uid", deviceModel.UID)
	ctx.WithValue("language", language)
	ctx.Request().Next()
}

// clientAuthTTL is how long a successful bcrypt verification is remembered.
const clientAuthTTL = 10 * time.Minute

// checkClientPassword verifies the presented password against the stored
// bcrypt hash, but only runs bcrypt once per client per clientAuthTTL.
//
// bcrypt at cost 12 costs a few hundred milliseconds of CPU, and the /api
// routes authenticate on every single request, so a busy client would spend
// most of the process' CPU re-deriving the same hash. After the first success
// an HMAC-SHA256 of (stored hash, presented password), keyed with the
// application key, is cached; a later request that presents the same password
// matches it in constant time and skips bcrypt entirely.
//
// The cached value is a keyed digest, never the password itself, and it is
// dropped together with the client cache by ClientService.ForgetCache, so a
// password reset or a status change takes effect immediately.
func checkClientPassword(username, presented, storedHash string) bool {
	key := services.ClientAuthCacheKey(username)
	digest := clientAuthDigest(storedHash, presented)

	if cached, ok := facades.Cache().Get(key).(string); ok && cached != "" &&
		hmac.Equal([]byte(cached), []byte(digest)) {
		return true
	}

	if !facades.Hash().Check(presented, storedHash) {
		return false
	}

	facades.Cache().Put(key, digest, clientAuthTTL)

	return true
}

// clientAuthDigest binds the stored hash and the presented password together
// under the application key, so the cached value is useless anywhere else and
// changes the moment either half changes.
func clientAuthDigest(storedHash, presented string) string {
	mac := hmac.New(sha256.New, []byte(facades.Config().GetString("app.key")))
	mac.Write([]byte(storedHash))
	mac.Write([]byte{0})
	mac.Write([]byte(presented))

	return hex.EncodeToString(mac.Sum(nil))
}

func touchDevice(ctx http.Context, clientID uint, uid string) (models.Device, error) {
	device := models.Device{}
	values := map[string]any{
		"last_seen_at": carbon.NewDateTime(carbon.Now()),
		"ip":           ctx.Request().Ip(),
	}

	platform := ctx.Request().Headers().Get("x-platform")
	switch platform {
	case "android", "ios", "web":
	default:
		platform = "unknown"
	}
	values["platform"] = platform
	if version := ctx.Request().Headers().Get("x-app-version"); version != "" {
		values["app_version"] = version
	}
	if token := ctx.Request().Headers().Get("x-fcm-token"); token != "" {
		if err := detachToken(token, clientID, uid); err != nil {
			return device, err
		}
		values["fcm_token"] = token
	}

	err := facades.Orm().Query().UpdateOrCreate(&device, models.Device{UID: uid, ClientID: clientID}, values)
	if err != nil && isUniqueViolation(err) {
		// Two requests from the same device can race on the very first call and
		// both try to insert the row. The loser sees a unique violation; the
		// row it wanted exists by now, so read it back instead of failing.
		device = models.Device{}
		if readErr := facades.Orm().Query().
			Where("uid", uid).Where("client_id", clientID).First(&device); readErr == nil && device.ID != 0 {
			return device, nil
		}
	}

	return device, err
}

// isUniqueViolation reports whether a database error is a duplicate key error.
// Postgres reports SQLSTATE 23505 for it.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())

	return strings.Contains(message, "23505") ||
		strings.Contains(message, "duplicate key value") ||
		strings.Contains(message, "unique constraint")
}

func detachToken(token string, clientID uint, uid string) error {
	_, err := facades.Orm().Query().Model(&models.Device{}).
		Where("fcm_token", token).
		Where(func(query orm.Query) orm.Query {
			return query.Where("uid != ?", uid).OrWhere("client_id != ?", clientID)
		}).
		Update("fcm_token", nil)

	return err
}

// abort answers with the API wide error envelope, {"message": "..."}, and
// stops the middleware chain. Status codes are unchanged.
func abort(ctx http.Context, status int, message string) {
	ctx.Request().AbortWithStatusJson(status, http.Json{"message": message})
}
