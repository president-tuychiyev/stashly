package middleware

import (
	"mime"
	"path"
	"strings"

	"github.com/goravel/framework/contracts/http"

	"github.com/president-tuychiyev/stashly/api/app/services"
)

// executableTypes are the media types a browser will happily run as code from
// the origin that served them. A stored .html or .svg on the public disk would
// otherwise be a stored XSS against this service's own origin: it can read the
// cookies of the panel and call the admin API as the logged in user.
var executableTypes = map[string]bool{
	"text/html":             true,
	"application/xhtml+xml": true,
	"image/svg+xml":         true,
	"text/xml":              true,
	"application/xml":       true,
	"application/xslt+xml":  true,
	"text/xsl":              true,
}

// SafeStatic hardens the /storage listing of public files.
//
// Every response gets X-Content-Type-Options: nosniff so a mislabelled file is
// not re-interpreted by the browser. Anything whose type can execute is forced
// to download with Content-Disposition: attachment and additionally neutered
// with Content-Security-Policy: sandbox, which drops scripts, plugins, forms
// and the same-origin privilege even if the file is opened directly. Every
// other type stays inline, so images and PDFs still render in the browser.
func SafeStatic() http.Middleware {
	return &safeStatic{}
}

type safeStatic struct{}

func (r *safeStatic) Signature() string {
	return "safe_static"
}

func (r *safeStatic) Handle(ctx http.Context) {
	name := path.Base(ctx.Request().Path())
	mediaType := strings.ToLower(strings.TrimSpace(mime.TypeByExtension(strings.ToLower(path.Ext(name)))))
	if index := strings.Index(mediaType, ";"); index > 0 {
		mediaType = strings.TrimSpace(mediaType[:index])
	}

	response := ctx.Response()
	response.Header("X-Content-Type-Options", "nosniff")

	if executableTypes[mediaType] {
		response.Header("Content-Disposition", services.ContentDisposition(name))
		response.Header("Content-Security-Policy", "sandbox")
	} else {
		response.Header("Content-Disposition", "inline")
	}

	ctx.Request().Next()
}
