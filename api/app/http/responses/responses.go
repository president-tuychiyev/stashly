// Package responses centralises the JSON envelopes used by the whole API so
// that every endpoint answers with the same shape.
package responses

import (
	"math"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/validation"
)

// Meta describes a page of a paginated listing.
type Meta struct {
	Page     int   `json:"page"`
	PerPage  int   `json:"per_page"`
	Total    int64 `json:"total"`
	LastPage int   `json:"last_page"`
}

// Pagination holds the page/per_page pair taken from the query string.
type Pagination struct {
	Page    int
	PerPage int
}

// ReadPagination reads "page" and "per_page" from the query string and clamps
// them to sane values (per_page is capped at 100).
func ReadPagination(ctx http.Context) Pagination {
	page := ctx.Request().QueryInt("page", 1)
	if page < 1 {
		page = 1
	}

	perPage := ctx.Request().QueryInt("per_page", 20)
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	return Pagination{Page: page, PerPage: perPage}
}

// NewMeta builds the meta block for a page of results.
func NewMeta(p Pagination, total int64) Meta {
	lastPage := int(math.Ceil(float64(total) / float64(p.PerPage)))
	if lastPage < 1 {
		lastPage = 1
	}

	return Meta{Page: p.Page, PerPage: p.PerPage, Total: total, LastPage: lastPage}
}

// Data answers with a single resource wrapped in a "data" key.
func Data(ctx http.Context, status int, data any) http.Response {
	return ctx.Response().Status(status).Json(http.Json{"data": data})
}

// Paginated answers with a list of resources plus its meta block.
func Paginated(ctx http.Context, data any, p Pagination, total int64) http.Response {
	return ctx.Response().Success().Json(http.Json{"data": data, "meta": NewMeta(p, total)})
}

// NoContent answers with an empty 204.
func NoContent(ctx http.Context) http.Response {
	return ctx.Response().NoContent()
}

// Error answers with the standard error envelope.
func Error(ctx http.Context, status int, message string) http.Response {
	return ctx.Response().Status(status).Json(http.Json{"message": message})
}

// NotFound is the answer used whenever a resource does not exist or does not
// belong to the caller.
func NotFound(ctx http.Context) http.Response {
	return Error(ctx, http.StatusNotFound, "not found")
}

// Invalid answers with 422 and a map of per-field messages.
func Invalid(ctx http.Context, errors map[string][]string) http.Response {
	return ctx.Response().Status(http.StatusUnprocessableEntity).Json(http.Json{
		"message": "the given data was invalid",
		"errors":  errors,
	})
}

// InvalidField is a shortcut for a single field error.
func InvalidField(ctx http.Context, field, message string) http.Response {
	return Invalid(ctx, map[string][]string{field: {message}})
}

// FromValidationErrors converts Goravel validation errors into the API shape.
func FromValidationErrors(ctx http.Context, errs validation.Errors) http.Response {
	out := map[string][]string{}
	for field, messages := range errs.All() {
		for _, message := range messages {
			out[field] = append(out[field], message)
		}
	}

	return Invalid(ctx, out)
}
