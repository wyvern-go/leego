package leego

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"runtime"
	"sync"

	"golang.org/x/net/context"

	"github.com/go-wyvern/leego/engine"
	"github.com/go-wyvern/leego/utils"
	"github.com/go-wyvern/logger"
)

type (
	// Leego is the top-level framework instance.
	Leego struct {
		premiddleware      []MiddlewareFunc
		middleware         []MiddlewareFunc
		maxParam           *int
		wg                 utils.WaitGroupWrapper
		notFoundHandler    HandlerFunc
		httpErrorHandler   HTTPErrorHandler
		httpSuccessHandler HTTPSuccessHandler
		binder             Binder
		renderer           Renderer
		pool               sync.Pool
		debug              bool
		router             *Router
		logger             *logger.Logger
	}

	// Route contains a handler and information for matching against requests.
	Route struct {
		Method  string
		Path    string
		Handler string
	}

	// HTTPError represents an error that occurred while handling a request.
	HTTPError struct {
		Code    int
		Message string
	}

	// MiddlewareFunc defines a function to process middleware.
	MiddlewareFunc func(HandlerFunc) HandlerFunc

	// HandlerFunc defines a function to server HTTP requests.
	HandlerFunc func(Context) LeegoError

	// HTTPErrorHandler is a centralized HTTP error handler.
	HTTPErrorHandler   func(LeegoError, Context)
	HTTPSuccessHandler func(Context)

	ResponseHandler func(LeegoError, Context)

	LeegoError interface {
		Error() string
	}

	// Validator is the interface that wraps the Validate function.
	Validator interface {
		Validate() error
	}

	// Renderer is the interface that wraps the Render function.
	Renderer interface {
		Render(io.Writer, string, interface{}, Context) error
	}
)

// HTTP methods
const (
	CONNECT = "CONNECT"
	DELETE  = "DELETE"
	GET     = "GET"
	HEAD    = "HEAD"
	OPTIONS = "OPTIONS"
	PATCH   = "PATCH"
	POST    = "POST"
	PUT     = "PUT"
	TRACE   = "TRACE"
)

var (
	methods = [...]string{
		CONNECT,
		DELETE,
		GET,
		HEAD,
		OPTIONS,
		PATCH,
		POST,
		PUT,
		TRACE,
	}
)

// MIME types
const (
	MIMEApplicationJSON                  = "application/json"
	MIMEApplicationJSONCharsetUTF8       = MIMEApplicationJSON + "; " + charsetUTF8
	MIMEApplicationJavaScript            = "application/javascript"
	MIMEApplicationJavaScriptCharsetUTF8 = MIMEApplicationJavaScript + "; " + charsetUTF8
	MIMEApplicationXML                   = "application/xml"
	MIMEApplicationXMLCharsetUTF8        = MIMEApplicationXML + "; " + charsetUTF8
	MIMEApplicationForm                  = "application/x-www-form-urlencoded"
	MIMEApplicationProtobuf              = "application/protobuf"
	MIMEApplicationMsgpack               = "application/msgpack"
	MIMETextHTML                         = "text/html"
	MIMETextHTMLCharsetUTF8              = MIMETextHTML + "; " + charsetUTF8
	MIMETextPlain                        = "text/plain"
	MIMETextPlainCharsetUTF8             = MIMETextPlain + "; " + charsetUTF8
	MIMEMultipartForm                    = "multipart/form-data"
	MIMEOctetStream                      = "application/octet-stream"
)

const (
	charsetUTF8 = "charset=utf-8"
)

// Headers
const (
	HeaderAcceptEncoding                = "Accept-Encoding"
	HeaderAllow                         = "Allow"
	HeaderAuthorization                 = "Authorization"
	HeaderContentDisposition            = "Content-Disposition"
	HeaderContentEncoding               = "Content-Encoding"
	HeaderContentLength                 = "Content-Length"
	HeaderContentType                   = "Content-Type"
	HeaderCookie                        = "Cookie"
	HeaderSetCookie                     = "Set-Cookie"
	HeaderIfModifiedSince               = "If-Modified-Since"
	HeaderLastModified                  = "Last-Modified"
	HeaderLocation                      = "Location"
	HeaderUpgrade                       = "Upgrade"
	HeaderVary                          = "Vary"
	HeaderWWWAuthenticate               = "WWW-Authenticate"
	HeaderXForwardedProto               = "X-Forwarded-Proto"
	HeaderXHTTPMethodOverride           = "X-HTTP-Method-Override"
	HeaderXForwardedFor                 = "X-Forwarded-For"
	HeaderXRealIP                       = "X-Real-IP"
	HeaderServer                        = "Server"
	HeaderOrigin                        = "Origin"
	HeaderAccessControlRequestMethod    = "Access-Control-Request-Method"
	HeaderAccessControlRequestHeaders   = "Access-Control-Request-Headers"
	HeaderAccessControlAllowOrigin      = "Access-Control-Allow-Origin"
	HeaderAccessControlAllowMethods     = "Access-Control-Allow-Methods"
	HeaderAccessControlAllowHeaders     = "Access-Control-Allow-Headers"
	HeaderAccessControlAllowCredentials = "Access-Control-Allow-Credentials"
	HeaderAccessControlExposeHeaders    = "Access-Control-Expose-Headers"
	HeaderAccessControlMaxAge           = "Access-Control-Max-Age"

	// Security
	HeaderStrictTransportSecurity = "Strict-Transport-Security"
	HeaderXContentTypeOptions     = "X-Content-Type-Options"
	HeaderXXSSProtection          = "X-XSS-Protection"
	HeaderXFrameOptions           = "X-Frame-Options"
	HeaderContentSecurityPolicy   = "Content-Security-Policy"
	HeaderXCSRFToken              = "X-CSRF-Token"
)

// Errors
var (
	ErrUnsupportedMediaType        = NewHTTPError(http.StatusUnsupportedMediaType)
	ErrNotFound                    = NewHTTPError(http.StatusNotFound)
	ErrUnauthorized                = NewHTTPError(http.StatusUnauthorized)
	ErrMethodNotAllowed            = NewHTTPError(http.StatusMethodNotAllowed)
	ErrStatusRequestEntityTooLarge = NewHTTPError(http.StatusRequestEntityTooLarge)
	ErrRendererNotRegistered       = errors.New("renderer not registered")
	ErrInvalidRedirectCode         = errors.New("invalid redirect status code")
	ErrCookieNotFound              = errors.New("cookie not found")
)

// Error handlers
var (
	NotFoundHandler = func(c Context) LeegoError {
		return ErrNotFound
	}

	MethodNotAllowedHandler = func(c Context) LeegoError {
		return ErrMethodNotAllowed
	}
)

// NewHTTPError creates a new HTTPError instance.
func NewHTTPError(code int, msg ...string) *HTTPError {
	he := &HTTPError{Code: code, Message: http.StatusText(code)}
	if len(msg) > 0 {
		m := msg[0]
		he.Message = m
	}
	return he
}

// Error makes it compatible with `error` interface.
func (e *HTTPError) Error() string {
	return e.Message
}

// New creates an instance of Echo.
func New() (e *Leego) {
	e = &Leego{maxParam: new(int)}
	e.pool.New = func() interface{} {
		return e.NewContext(nil, nil)
	}
	e.router = NewRouter(e)

	e.SetBinder(&binder{})
	e.SetHTTPErrorHandler(e.DefaultHTTPErrorHandler)
	e.SetHTTPSuccessHandler(e.DefaultHTTPSuccessHandler)
	return
}

// NewContext returns a Context instance.
func (e *Leego) NewContext(req engine.Request, res engine.Response) Context {
	return &echoContext{
		context:  context.Background(),
		request:  req,
		response: res,
		leego:    e,
		pvalues:  make([]string, *e.maxParam),
		handler:  NotFoundHandler,
		data:     make(map[string]interface{}),
	}
}

func (e *Leego) ResponseHandler(err LeegoError, c Context) {
	if err != nil {
		e.httpErrorHandler(err, c)
	} else {
		e.httpSuccessHandler(c)
	}

}

// Router returns router.
func (e *Leego) Router() *Router {
	return e.router
}

// DefaultHTTPErrorHandler invokes the default HTTP error handler.
func (e *Leego) DefaultHTTPErrorHandler(err LeegoError, c Context) {
	code := http.StatusInternalServerError
	msg := http.StatusText(code)
	if he, ok := err.(*HTTPError); ok {
		code = he.Code
		msg = he.Message
	}
	if e.debug {
		msg = err.Error()
	}
	if !c.Response().Committed() {
		if c.Request().Method() == HEAD {
			// Issue #608
			c.NoContent(code)
		} else {
			c.String(code, msg)
		}
	}
}

func (e *Leego) DefaultHTTPSuccessHandler(c Context) {}

func (e *Leego) SetHTTPErrorHandler(h HTTPErrorHandler) {
	e.httpErrorHandler = h
}

func (e *Leego) SetHTTPSuccessHandler(h HTTPSuccessHandler) {
	e.httpSuccessHandler = h
}

// SetBinder registers a custom binder. It's invoked by `Context#Bind()`.
func (e *Leego) SetBinder(b Binder) {
	e.binder = b
}

// Binder returns the binder instance.
func (e *Leego) Binder() Binder {
	return e.binder
}

// Pre adds middleware to the chain which is run before router.
func (e *Leego) Pre(middleware ...MiddlewareFunc) {
	e.premiddleware = append(e.premiddleware, middleware...)
}

// Use adds middleware to the chain which is run after router.
func (e *Leego) Use(middleware ...MiddlewareFunc) {
	e.middleware = append(e.middleware, middleware...)
}

// CONNECT registers a new CONNECT route for a path with matching handler in the
// router with optional route-level middleware.
func (e *Leego) CONNECT(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.add(CONNECT, path, h, m...)
}

// Connect is deprecated, use `CONNECT()` instead.
func (e *Leego) Connect(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.CONNECT(path, h, m...)
}

// DELETE registers a new DELETE route for a path with matching handler in the router
// with optional route-level middleware.
func (e *Leego) DELETE(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.add(DELETE, path, h, m...)
}

// Delete is deprecated, use `DELETE()` instead.
func (e *Leego) Delete(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.DELETE(path, h, m...)
}

// GET registers a new GET route for a path with matching handler in the router
// with optional route-level middleware.
func (e *Leego) GET(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.add(GET, path, h, m...)
}

// Get is deprecated, use `GET()` instead.
func (e *Leego) Get(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.GET(path, h, m...)
}

// HEAD registers a new HEAD route for a path with matching handler in the
// router with optional route-level middleware.
func (e *Leego) HEAD(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.add(HEAD, path, h, m...)
}

// Head is deprecated, use `HEAD()` instead.
func (e *Leego) Head(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.HEAD(path, h, m...)
}

// OPTIONS registers a new OPTIONS route for a path with matching handler in the
// router with optional route-level middleware.
func (e *Leego) OPTIONS(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.add(OPTIONS, path, h, m...)
}

// Options is deprecated, use `OPTIONS()` instead.
func (e *Leego) Options(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.OPTIONS(path, h, m...)
}

// PATCH registers a new PATCH route for a path with matching handler in the
// router with optional route-level middleware.
func (e *Leego) PATCH(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.add(PATCH, path, h, m...)
}

// Patch is deprecated, use `PATCH()` instead.
func (e *Leego) Patch(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.PATCH(path, h, m...)
}

// POST registers a new POST route for a path with matching handler in the
// router with optional route-level middleware.
func (e *Leego) POST(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.add(POST, path, h, m...)
}

// Post is deprecated, use `POST()` instead.
func (e *Leego) Post(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.POST(path, h, m...)
}

// PUT registers a new PUT route for a path with matching handler in the
// router with optional route-level middleware.
func (e *Leego) PUT(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.add(PUT, path, h, m...)
}

// Put is deprecated, use `PUT()` instead.
func (e *Leego) Put(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.PUT(path, h, m...)
}

// TRACE registers a new TRACE route for a path with matching handler in the
// router with optional route-level middleware.
func (e *Leego) TRACE(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.add(TRACE, path, h, m...)
}

// Trace is deprecated, use `TRACE()` instead.
func (e *Leego) Trace(path string, h HandlerFunc, m ...MiddlewareFunc) {
	e.TRACE(path, h, m...)
}

// Any registers a new route for all HTTP methods and path with matching handler
// in the router with optional route-level middleware.
func (e *Leego) Any(path string, handler HandlerFunc, middleware ...MiddlewareFunc) {
	for _, m := range methods {
		e.add(m, path, handler, middleware...)
	}
}

// Match registers a new route for multiple HTTP methods and path with matching
// handler in the router with optional route-level middleware.
func (e *Leego) Match(methods []string, path string, handler HandlerFunc, middleware ...MiddlewareFunc) {
	for _, m := range methods {
		e.add(m, path, handler, middleware...)
	}
}

func (e *Leego) Add(method, path string, handler HandlerFunc, middleware ...MiddlewareFunc) {
	e.add(method, path, handler, middleware...)
}

func (e *Leego) add(method, path string, handler HandlerFunc, middleware ...MiddlewareFunc) {
	name := handlerName(handler)
	e.router.Add(method, path, func(c Context) LeegoError {
		h := handler
		// Chain middleware
		for i := len(middleware) - 1; i >= 0; i-- {
			h = middleware[i](h)
		}
		return h(c)
	}, e)
	r := Route{
		Method:  method,
		Path:    path,
		Handler: name,
	}

	e.router.routes[method+path] = r
}

// Logger returns the logger instance.
func (e *Leego) Logger() *logger.Logger {
	return e.logger
}

// SetLogger defines a custom logger.
func (e *Leego) SetLogger(l *logger.Logger) {
	e.logger = l
}

func (e *Leego) ServeHTTP(req engine.Request, res engine.Response) {
	c := e.pool.Get().(*echoContext)
	c.Reset(req, res)
	c.SetLang(req.Header().Get("Accept-Language"))

	// Middleware
	h := func(Context) LeegoError {
		method := req.Method()
		path := req.URL().Path()
		e.router.Find(method, path, c)
		h := c.handler
		for i := len(e.middleware) - 1; i >= 0; i-- {
			h = e.middleware[i](h)
		}
		return h(c)
	}

	// Premiddleware
	for i := len(e.premiddleware) - 1; i >= 0; i-- {
		h = e.premiddleware[i](h)
	}

	// Execute chain
	err := h(c)
	e.ResponseHandler(err, c)

	e.pool.Put(c)
}

// Run starts the HTTP server.
func (e *Leego) Run(s engine.Server) {
	s.SetLogger(e.logger)
	s.SetHandler(e)
	s.Start()
}

// Group creates a new router group with prefix and optional group-level middleware.
func (e *Leego) Group(prefix string, m ...MiddlewareFunc) (g *Group) {
	g = &Group{prefix: prefix, leego: e}
	g.Use(m...)
	return
}

// URI generates a URI from handler.
func (e *Leego) URI(handler HandlerFunc, params ...interface{}) string {
	uri := new(bytes.Buffer)
	ln := len(params)
	n := 0
	name := handlerName(handler)
	for _, r := range e.router.routes {
		if r.Handler == name {
			for i, l := 0, len(r.Path); i < l; i++ {
				if r.Path[i] == ':' && n < ln {
					for ; i < l && r.Path[i] != '/'; i++ {
					}
					uri.WriteString(fmt.Sprintf("%v", params[n]))
					n++
				}
				if i < l {
					uri.WriteByte(r.Path[i])
				}
			}
			break
		}
	}
	return uri.String()
}

// URL is an alias for `URI` function.
func (e *Leego) URL(h HandlerFunc, params ...interface{}) string {
	return e.URI(h, params...)
}

// WrapMiddleware wrap `echo.HandlerFunc` into `echo.MiddlewareFunc`.
func WrapMiddleware(h HandlerFunc) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c Context) LeegoError {
			if err := h(c); err != nil {
				return err
			}
			return next(c)
		}
	}
}

func handlerName(h HandlerFunc) string {
	t := reflect.ValueOf(h).Type()
	if t.Kind() == reflect.Func {
		return runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
	}
	return t.String()
}
