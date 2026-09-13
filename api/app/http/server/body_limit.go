// Package server holds the raw net/http layer that has to sit in front of
// Goravel's own middleware stack.
//
// Why this exists: goravel/gin turns every Goravel middleware into a gin
// handler through middlewareToGinHandler (gin utils.go), and the very first
// thing that wrapper does is build a Context and call
// context.Request().Info(). Building the ContextRequest runs getHttpBody
// (gin context_request.go), which calls ParseMultipartForm(32<<20) for a
// multipart request. In other words the whole body is already read - and
// spilled into temp files - before the first Goravel middleware Handle() ever
// runs, and Goravel additionally prepends its own global handlers (recover,
// timeout, cors, tls, maintenance) at engine level in Route.init(). A Goravel
// middleware therefore cannot cap an upload; only a plain net/http wrapper
// installed above the gin engine can.
//
// The gin engine itself is unexported (Route.instance) and the router group
// Goravel registers routes on snapshots the engine handler chain at init time,
// so prepending a gin.HandlerFunc later would not reach the routes either.
// What is reachable is the http.Server: WrapBodyLimit returns a route.Route
// that behaves exactly like the gin one but serves through its own
// http.Server whose handler caps the body first.
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	stdhttp "net/http"
	"net/http/httptest"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"

	"github.com/president-tuychiyev/stashly/api/app/facades"
)

// Slack is the head room added on top of the theoretical payload so that the
// multipart boundaries, the part headers and the extra form fields of a full
// request never trip the limit on their own.
const Slack = 1 << 20

const (
	defaultMaxFileSize        = 52428800
	defaultMaxFilesPerRequest = 20
)

// HardLimit is the largest request body the process accepts on any route. It
// is deliberately computed from the configuration only: this check runs before
// authentication, so it must not touch the database. The tighter, per client
// limit is applied afterwards by the limit_upload_size middleware.
func HardLimit() int64 {
	maxFileSize := int64(facades.Config().GetInt("storage.max_file_size"))
	if maxFileSize <= 0 {
		maxFileSize = defaultMaxFileSize
	}

	maxFiles := int64(facades.Config().GetInt("storage.max_files_per_request"))
	if maxFiles <= 0 {
		maxFiles = defaultMaxFilesPerRequest
	}

	return maxFileSize*maxFiles + Slack
}

type bodyStateKey struct{}

// bodyState records whether the capped body was actually overrun, so a
// middleware further down the chain can answer 413 instead of letting the
// controller report a mangled, half parsed form.
type bodyState struct {
	exceeded bool
}

// BodyLimitExceeded reports whether the body of this request hit the hard cap.
func BodyLimitExceeded(request *stdhttp.Request) bool {
	if request == nil {
		return false
	}

	state, ok := request.Context().Value(bodyStateKey{}).(*bodyState)

	return ok && state.exceeded
}

// limitedBody notices the MaxBytesError that http.MaxBytesReader returns and
// records it on the shared state.
type limitedBody struct {
	io.ReadCloser
	state *bodyState
}

func (r *limitedBody) Read(p []byte) (int, error) {
	read, err := r.ReadCloser.Read(p)

	var tooLarge *stdhttp.MaxBytesError
	if errors.As(err, &tooLarge) {
		r.state.exceeded = true
	}

	return read, err
}

// CapBody rejects an over long request outright and caps everything else.
// It reports whether the request may continue.
func CapBody(writer stdhttp.ResponseWriter, request *stdhttp.Request) bool {
	switch request.Method {
	case stdhttp.MethodPost, stdhttp.MethodPut, stdhttp.MethodPatch:
	default:
		return true
	}
	if request.Body == nil || request.Body == stdhttp.NoBody {
		return true
	}

	limit := HardLimit()
	if request.ContentLength > limit {
		Reject(writer)

		return false
	}

	state := &bodyState{}
	body := stdhttp.MaxBytesReader(writer, request.Body, limit)
	request.Body = &limitedBody{ReadCloser: body, state: state}
	*request = *request.WithContext(context.WithValue(request.Context(), bodyStateKey{}, state))

	return true
}

// Reject writes the API wide 413 envelope.
func Reject(writer stdhttp.ResponseWriter) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(http.StatusRequestEntityTooLarge)
	_, _ = writer.Write([]byte(`{"message":"file too large"}`))
}

// bodyLimitRoute is a route.Route that serves through its own http.Server so
// that the body cap runs before anything Goravel installs.
type bodyLimitRoute struct {
	route.Route
	server    *stdhttp.Server
	tlsServer *stdhttp.Server
}

// WrapBodyLimit decorates a driver route with the raw body cap.
func WrapBodyLimit(inner route.Route) route.Route {
	return &bodyLimitRoute{Route: inner}
}

func (r *bodyLimitRoute) ServeHTTP(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
	if !CapBody(writer, request) {
		return
	}

	r.Route.ServeHTTP(writer, request)
}

func (r *bodyLimitRoute) Test(request *stdhttp.Request) (*stdhttp.Response, error) {
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, request)

	return recorder.Result(), nil
}

func (r *bodyLimitRoute) handler() stdhttp.Handler {
	return stdhttp.AllowQuerySemicolons(stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
		if !CapBody(writer, request) {
			return
		}

		r.Route.ServeHTTP(writer, request)
	}))
}

func (r *bodyLimitRoute) newServer(addr string) *stdhttp.Server {
	driver := facades.Config().GetString("http.default", "gin")

	return &stdhttp.Server{
		Addr:           addr,
		Handler:        r.handler(),
		MaxHeaderBytes: facades.Config().GetInt(fmt.Sprintf("http.drivers.%s.header_limit", driver), 4096) << 10,
	}
}

func (r *bodyLimitRoute) Run(host ...string) error {
	addr, err := r.address(host, "http.host", "http.port")
	if err != nil {
		return err
	}

	fmt.Println("[HTTP] Listening on: http://" + addr)
	r.server = r.newServer(addr)

	return ignoreClosed(r.server.ListenAndServe())
}

func (r *bodyLimitRoute) RunTLS(host ...string) error {
	addr, err := r.address(host, "http.tls.host", "http.tls.port")
	if err != nil {
		return err
	}

	return r.RunTLSWithCert(addr,
		facades.Config().GetString("http.tls.ssl.cert"),
		facades.Config().GetString("http.tls.ssl.key"),
	)
}

func (r *bodyLimitRoute) RunTLSWithCert(host, certFile, keyFile string) error {
	if host == "" {
		return errors.New("host can't be empty")
	}
	if certFile == "" || keyFile == "" {
		return errors.New("certificate can't be empty")
	}

	fmt.Println("[HTTPS] Listening on: https://" + host)
	r.tlsServer = r.newServer(host)

	return ignoreClosed(r.tlsServer.ListenAndServeTLS(certFile, keyFile))
}

func (r *bodyLimitRoute) Listen(l net.Listener) error {
	fmt.Println("[HTTP] Listening on: http://" + l.Addr().String())
	r.server = r.newServer(l.Addr().String())

	return ignoreClosed(r.server.Serve(l))
}

func (r *bodyLimitRoute) ListenTLS(l net.Listener) error {
	return r.ListenTLSWithCert(l,
		facades.Config().GetString("http.tls.ssl.cert"),
		facades.Config().GetString("http.tls.ssl.key"),
	)
}

func (r *bodyLimitRoute) ListenTLSWithCert(l net.Listener, certFile, keyFile string) error {
	fmt.Println("[HTTPS] Listening on: https://" + l.Addr().String())
	r.tlsServer = r.newServer(l.Addr().String())

	return ignoreClosed(r.tlsServer.ServeTLS(l, certFile, keyFile))
}

func (r *bodyLimitRoute) Shutdown(ctx ...context.Context) error {
	c := context.Background()
	if len(ctx) > 0 {
		c = ctx[0]
	}

	if r.server != nil {
		return r.server.Shutdown(c)
	}
	if r.tlsServer != nil {
		return r.tlsServer.Shutdown(c)
	}

	return r.Route.Shutdown(ctx...)
}

func (r *bodyLimitRoute) address(host []string, hostKey, portKey string) (string, error) {
	if len(host) > 0 && host[0] != "" {
		return host[0], nil
	}

	port := facades.Config().GetString(portKey)
	if port == "" {
		return "", errors.New("port can't be empty")
	}

	return facades.Config().GetString(hostKey) + ":" + port, nil
}

func ignoreClosed(err error) error {
	if errors.Is(err, stdhttp.ErrServerClosed) {
		return nil
	}

	return err
}
