package rest

import (
	jwtops "bigbucks/solution/auth/jwt-ops"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/request_context"
	"bigbucks/solution/auth/settings"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	//Load all controllers methods by deafult

	"go.uber.org/zap"
)

// responseLogger is wrapper of http.ResponseWriter that keeps track of its HTTP
// status code and body size
type responseLogger struct {
	w      http.ResponseWriter
	status int
	size   int
}

func (l *responseLogger) Write(b []byte) (int, error) {
	size, err := l.w.Write(b)
	l.size += size
	return size, err
}

func (l *responseLogger) WriteHeader(s int) {
	l.w.WriteHeader(s)
	l.status = s
}

func (l *responseLogger) Header() http.Header {
	return l.w.Header()
}

func (l *responseLogger) Status() int {
	return l.status
}

func (l *responseLogger) Size() int {
	return l.size
}

func Authenticate(w http.ResponseWriter, r *http.Request, settings *settings.Settings) (bool, settings.AuthToken, error) {

	// updated := d.store.Users.LastUpdate(tk.User.ID) > tk.IssuedAt
	authToken, _, err := jwtops.VerifyJWT(r)
	if err != nil {
		return false, authToken, err
	}

	expired := !authToken.VerifyExpiresAt(time.Now().Add(time.Hour), true)
	if expired {
		w.Header().Add("X-Renew-Token", "true")
	}

	return true, authToken, err

}
func JSONError(w http.ResponseWriter, err interface{}, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	err = json.NewEncoder(w).Encode(err)
	if err != nil {
		loging.Logger.Errorln(err)
	}
}

func handle(fn handleFunc, config *handlerConfig, setting *settings.Settings, perm_cache *permission_cache.PermissionCache) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_responseLogger := &responseLogger{w: w, status: http.StatusOK}
		ctx := &request_context.Context{}
		ctx.Settings = setting
		ctx.Context = context.TODO()
		if config.auth {
			success, authToken, _ := Authenticate(w, r, setting)
			loging.Logger.Debugln(success, zap.String("Authenticated User", authToken.User.Username))
			if !success {
				http.Error(w, strconv.Itoa(http.StatusUnauthorized)+" "+http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			ctx.Auth = &authToken
		}
		// if config.resource != "" && config.scope != "" && config.action != "" {
		// 	hasPermission, err := perm_cache.PermCache.CheckPermission(config.resource, config.scope, config.action, &ctx.Auth.User)
		// 	if err != nil || !hasPermission {
		// 		http.Error(w, "Forbidden", http.StatusForbidden)
		// 		return
		// 	}
		// }
		start := time.Now()
		status, err := fn(_responseLogger, r, ctx)

		if status != 0 {
			JSONError(w, err, status)
		}
		loging.Logger.Infow("Request:",
			zap.String("proto", r.Proto),
			zap.String("method", r.Method),
			zap.String("endpoint", r.URL.String()),
			zap.String("user-agent", r.UserAgent()),
			zap.String("remote-addr", r.RemoteAddr),
			zap.Int("status", _responseLogger.Status()),
			zap.String("latency", time.Since(start).String()),
			zap.Int("size", _responseLogger.Size()),
		)
	})

	return http.StripPrefix(config.prefix, handler)
}
