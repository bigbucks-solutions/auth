package http

import (
	"log"
	"net/http"
	"strconv"
	"github.com/bigbucks/solution/auth/settings"
)

func handle(fn handleFunc, prefix string, server *settings.Server) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, err := fn(w, r)

		if status != 0 {
			txt := http.StatusText(status)
			http.Error(w, strconv.Itoa(status)+" "+txt, status)
		}

		if status >= 400 || err != nil {
			log.Printf("%s: %v %s %v", r.URL.Path, status, r.RemoteAddr, err)
		}
	})

	return http.StripPrefix(prefix, handler)
}