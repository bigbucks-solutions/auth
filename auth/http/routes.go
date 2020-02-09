package http

import (
	"net/http"
	"github.com/gorilla/mux"
	"github.com/bigbucks/solution/auth/settings"
)

type handleFunc func(w http.ResponseWriter, r *http.Request) (int, error)

// NewHandler Provide Http handler
func NewHandler(server *settings.Server) (http.Handler, error) {
	server.Clean()

	r := mux.NewRouter()

	monkey := func(fn handleFunc, prefix string) http.Handler {
		return handle(fn, prefix, server)
	}
	api := r.PathPrefix("/api").Subrouter()

	api.Handle("/posts", monkey(getPosts, "")).Methods("GET")


	return http.StripPrefix(server.BaseURL, r), nil
}
