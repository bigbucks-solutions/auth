package http

import (
	ctr "bigbucks/solution/auth/http/controllers" //Load all controllers methods by deafult
	"bigbucks/solution/auth/settings"
	"github.com/gorilla/mux"
	"net/http"
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
	api.Handle("/create-org", patch(ctr.CreateOrg, "", false)).Methods("POST")

	return http.StripPrefix(settings.BaseURL, r), nil
}
