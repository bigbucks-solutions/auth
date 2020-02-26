package http

import (
	"net/http"
	"github.com/gorilla/mux"
	"bigbucks/solution/auth/settings"
	ctr "bigbucks/solution/auth/http/controllers" //Load all controllers methods by deafult
)

type handleFunc func(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error)

// NewHandler Provide Http handler
func NewHandler(settings *settings.Settings) (http.Handler, error) {
	settings.Clean()

	r := mux.NewRouter()

	patch := func(fn handleFunc, prefix string, auth bool) http.Handler {
		return handle(fn, prefix, auth, settings)
	}
	api := r.PathPrefix("/api").Subrouter()
	api.Handle("/signin", patch(ctr.Signin, "", false)).Methods("POST")
	api.Handle("/renew", patch(ctr.RenewToken, "", true)).Methods("POST")
	api.Handle("/posts", patch(ctr.GetPosts, "", false)).Methods("GET")


	return http.StripPrefix(settings.BaseURL, r), nil
}
