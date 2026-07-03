package api

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/mobazha/mobazha/pkg/response"
)

// panicRecoveryMiddleware catches panics in downstream handlers and
// middleware, logs a full stack trace, and returns a 500 response
// instead of silently closing the HTTP connection.
//
// Without this middleware, Go's net/http server catches panics but
// simply closes the connection (HTTP/1.1) or sends a GOAWAY (HTTP/2),
// which surfaces as "Empty reply from server" for clients — with no
// application-level log entry. TD-104 demonstrated that this makes
// Sovereign runtime failures nearly impossible to diagnose.
func panicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			if rv := recover(); rv != nil {
				stack := debug.Stack()
				log.Errorf("PANIC: %s %s — %v\n%s", r.Method, r.URL.Path, rv, stack)

				if ww.Status() == 0 {
					response.Error(ww, http.StatusInternalServerError,
						response.CodeInternalError,
						fmt.Sprintf("internal server error (panic recovered)"))
				}
			}
		}()
		next.ServeHTTP(ww, r)
	})
}
