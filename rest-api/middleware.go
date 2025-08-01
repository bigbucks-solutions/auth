package rest

import (
	jwtops "bigbucks/solution/auth/jwt-ops"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/request_context"
	sessionstore "bigbucks/solution/auth/session_store"
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
func JSONError(responselogger *responseLogger, err interface{}, code int) {
	responselogger.Header().Set("Content-Type", "application/json; charset=utf-8")
	responselogger.Header().Set("X-Content-Type-Options", "nosniff")
	responselogger.WriteHeader(code)
	err = json.NewEncoder(responselogger.w).Encode(err)
	if err != nil {
		loging.Logger.Errorln(err)
	}
}

func handle(fn handleFunc, config *handlerConfig, setting *settings.Settings, perm_cache *permission_cache.PermissionCache, session_store *sessionstore.SessionStore) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		_responseLogger := &responseLogger{w: w, status: http.StatusOK}
		start := time.Now()
		defer func() {
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
		}()
		ctx := &request_context.Context{}
		ctx.Settings = setting
		ctx.Context = context.TODO()
		ctx.PermCache = perm_cache
		ctx.SessionStore = session_store
		if config.auth {
			success, authToken, _ := Authenticate(w, r, setting)
			loging.Logger.Debugw("Authenticated User: ", zap.String("User", authToken.ID))
			if !success {
				http.Error(w, strconv.Itoa(http.StatusUnauthorized)+" "+http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			ctx.Auth = &authToken
			orgID := r.Header.Get("X-Organization-Id")
			if orgID == "" {
				orgID = r.PathValue("org_id")
			}
			ctx.CurrentOrgID = orgID

			if config.resource != "" && config.scope != "" && config.action != "" {
				valid, _, err := session_store.ValidateSession(ctx.Auth.ID)
				if err != nil || !valid {
					http.Error(w, "Session expired", http.StatusUnauthorized)
					return
				}
				if ctx.CurrentOrgID == "" {
					loging.Logger.Warn("No org_id found in request")
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				hasPermission, err := perm_cache.CheckPermission(&ctx.Context, config.resource, config.scope, config.action, orgID, &ctx.Auth.User)
				if err != nil || !hasPermission {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
			}
		}

		status, err := fn(_responseLogger, r, ctx)

		if status != 0 {
			JSONError(_responseLogger, err, status)
		}

	})
	return http.StripPrefix(config.prefix, handler)
}
