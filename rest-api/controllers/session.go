package controllers

import (
	"bigbucks/solution/auth/request_context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// Sessions godoc
//
//	@Summary		List User session for provided userId
//	@Description	List User sessions for provided userId
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			X-Auth	header	string	true	"Authorization"
//	@Param			user_id	path	string					true	"User ID"
//	@Success		200		{array}	map[string]interface{}	"list of user sessions"
//	@Failure		400		""
//	@Failure		404		""
//	@Failure		500		""
//	@Router			/sessions/users/{user_id} [get]
func Sessions(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	vars := mux.Vars(r)
	userId := vars["user_id"]

	user_sessions, err := ctx.SessionStore.ListUserSessions(userId)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(user_sessions)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// RevokeSession godoc
//
//	@Summary		Revoke a specific user session
//	@Description	Revokes a specific session by session ID
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			X-Auth	header	string	true	"Authorization"
//	@Param			session_id	path		string	true	"Session ID to revoke"
//	@Success		200			{object}	map[string]string
//	@Failure		400			""
//	@Failure		404			""
//	@Failure		500			""
//	@Router			/sessions/{session_id} [delete]
func RevokeSession(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	vars := mux.Vars(r)
	sessionID := vars["session_id"]

	if sessionID == "" {
		return http.StatusBadRequest, nil
	}

	err := ctx.SessionStore.RevokeSession(sessionID)
	if err != nil && err.Error() == "session not found" {
		return http.StatusNotFound, err
	} else if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]string{"message": "Session revoked successfully"})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// RevokeAllSessions godoc
//
//	@Summary		Revoke all user sessions except current
//	@Description	Revokes all sessions for a user except the current session
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			X-Auth	header	string	true	"Authorization"
//	@Param			user_id	path		string	true	"User ID"
//	@Success		200		{object}	map[string]string
//	@Failure		400		""
//	@Failure		404		""
//	@Failure		500		""
//	@Router			/users/{user_id}/sessions [delete]
func RevokeAllSessions(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	if userID == "" {
		return http.StatusBadRequest, nil
	}

	// Get the current session ID from query parameter
	currentSessionID := ctx.Auth.ID

	err := ctx.SessionStore.RevokeAllUserSessions(userID, currentSessionID)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]string{"message": "All sessions revoked successfully"})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
