package middleware

import (
	"github.com/go-wyvern/leego"
)

type (
	// TrailingSlashConfig defines the config for TrailingSlash middleware.
	TrailingSlashConfig struct {
		// Skipper defines a function to skip middleware.
		Skipper Skipper

		// Status code to be used when redirecting the request.
		// Optional, but when provided the request is redirected using this code.
		RedirectCode int `json:"redirect_code"`
	}
)

var (
	// DefaultTrailingSlashConfig is the default TrailingSlash middleware config.
	DefaultTrailingSlashConfig = TrailingSlashConfig{
		Skipper: defaultSkipper,
	}
)

// AddTrailingSlash returns a root level (before router) middleware which adds a
// trailing slash to the request `URL#Path`.
//
// Usage `Leego#Pre(AddTrailingSlash())`
func AddTrailingSlash() leego.MiddlewareFunc {
	return AddTrailingSlashWithConfig(TrailingSlashConfig{})
}

// AddTrailingSlashWithConfig returns a AddTrailingSlash middleware from config.
// See `AddTrailingSlash()`.
func AddTrailingSlashWithConfig(config TrailingSlashConfig) leego.MiddlewareFunc {
	// Defaults
	if config.Skipper == nil {
		config.Skipper = DefaultTrailingSlashConfig.Skipper
	}

	return func(next leego.HandlerFunc) leego.HandlerFunc {
		return func(c leego.Context) leego.LeegoError {
			if config.Skipper(c) {
				return next(c)
			}

			req := c.Request()
			url := req.URL()
			path := url.Path()
			qs := url.QueryString()
			if path != "/" && path[len(path)-1] != '/' {
				path += "/"
				uri := path
				if qs != "" {
					uri += "?" + qs
				}

				// Redirect
				if config.RedirectCode != 0 {
					return c.Redirect(config.RedirectCode, uri)
				}

				// Forward
				req.SetURI(uri)
				url.SetPath(path)
			}
			return next(c)
		}
	}
}

// RemoveTrailingSlash returns a root level (before router) middleware which removes
// a trailing slash from the request URI.
//
// Usage `Leego#Pre(RemoveTrailingSlash())`
func RemoveTrailingSlash() leego.MiddlewareFunc {
	return RemoveTrailingSlashWithConfig(TrailingSlashConfig{})
}

// RemoveTrailingSlashWithConfig returns a RemoveTrailingSlash middleware from config.
// See `RemoveTrailingSlash()`.
func RemoveTrailingSlashWithConfig(config TrailingSlashConfig) leego.MiddlewareFunc {
	// Defaults
	if config.Skipper == nil {
		config.Skipper = DefaultTrailingSlashConfig.Skipper
	}

	return func(next leego.HandlerFunc) leego.HandlerFunc {
		return func(c leego.Context) leego.LeegoError {
			if config.Skipper(c) {
				return next(c)
			}

			req := c.Request()
			url := req.URL()
			path := url.Path()
			qs := url.QueryString()
			l := len(path) - 1
			if l >= 0 && path != "/" && path[l] == '/' {
				path = path[:l]
				uri := path
				if qs != "" {
					uri += "?" + qs
				}

				// Redirect
				if config.RedirectCode != 0 {
					return c.Redirect(config.RedirectCode, uri)
				}

				// Forward
				req.SetURI(uri)
				url.SetPath(path)
			}
			return next(c)
		}
	}
}
