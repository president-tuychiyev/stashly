package middleware

import (
	stdhttp "net/http"

	"github.com/goravel/framework/contracts/http"
	"github.com/spf13/cast"

	"github.com/president-tuychiyev/stashly/api/app/facades"
	"github.com/president-tuychiyev/stashly/api/app/http/server"
	"github.com/president-tuychiyev/stashly/api/app/models"
)

// LimitUploadSize is the second, per client half of the upload cap.
//
// The first half lives in app/http/server: a Goravel middleware cannot stop an
// oversized body, because goravel/gin already parses the multipart form while
// it builds the Context that is handed to the very first middleware. The raw
// wrapper therefore enforces the process wide hard cap
// (storage.max_file_size * storage.max_files_per_request + 1 MB) before a byte
// is parsed.
//
// What is left for this middleware is the part that needs the authenticated
// caller: a client whose own max_file_size is lower than the global one gets
// the tighter limit here, and a body that already tripped the hard cap is
// turned into a clean 413 instead of a half parsed form.
func LimitUploadSize() http.Middleware {
	return &limitUploadSize{}
}

type limitUploadSize struct{}

func (r *limitUploadSize) Signature() string {
	return "limit_upload_size"
}

func (r *limitUploadSize) Handle(ctx http.Context) {
	request := ctx.Request().Origin()
	if request == nil {
		ctx.Request().Next()
		return
	}

	if server.BodyLimitExceeded(request) {
		tooLarge(ctx)
		return
	}

	limit := uploadLimit(ctx)
	if request.ContentLength > limit {
		tooLarge(ctx)
		return
	}

	// MaxBytesReader needs no ResponseWriter here: the nil interface simply
	// skips the "tell the server the request was too large" hook, the read
	// still fails once the limit is passed.
	request.Body = stdhttp.MaxBytesReader(nil, request.Body, limit)

	ctx.Request().Next()
}

func tooLarge(ctx http.Context) {
	ctx.Request().AbortWithStatusJson(http.StatusRequestEntityTooLarge, http.Json{"message": "file too large"})
}

// uploadLimit is the largest request body the current caller may send.
func uploadLimit(ctx http.Context) int64 {
	maxFileSize := int64(facades.Config().GetInt("storage.max_file_size"))
	if maxFileSize <= 0 {
		maxFileSize = 52428800
	}

	// api_check has already authenticated the client for /api routes, so its
	// own max_file_size can be honoured. Admin uploads fall back to the env.
	if clientID := cast.ToUint(ctx.Value("client_id")); clientID > 0 {
		var client models.Client
		if err := facades.Orm().Query().Where("id", clientID).First(&client); err == nil &&
			client.MaxFileSize != nil && *client.MaxFileSize > 0 {
			maxFileSize = *client.MaxFileSize
		}
	}

	maxFiles := int64(facades.Config().GetInt("storage.max_files_per_request"))
	if maxFiles <= 0 {
		maxFiles = 20
	}

	return maxFileSize*maxFiles + server.Slack
}
