package rest

import (
	ctr "bigbucks/solution/auth/rest-api/controllers" //Load all controllers methods by deafult
	"bigbucks/solution/auth/settings"
	"net/http"

	_ "bigbucks/solution/auth/docs"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	httpSwagger "github.com/swaggo/http-swagger"
)

type handleFunc func(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error)

// @title           BigBucks Solutions Auth Engine
// @version         0.0.1
// @description     This is REST api definitions.
// @termsOfService  http://swagger.io/terms/

// @contact.name   Jamsheed
// @contact.url    http://www.swagger.io/support
// @contact.email  jamsheed@nsmail.dev

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8000
// @BasePath  /api/v1

//@securityDefinitions.apikey JWTAuth
//@in header
//@name X-Auth

// NewHandler Provide Http handler
func NewHandler(settings *settings.Settings) (http.Handler, error) {
	settings.Clean()

	r := mux.NewRouter()
	

	patch := func(fn handleFunc, prefix string, auth bool) http.Handler {
		return handle(fn, prefix, auth, settings)
	}
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Handle("/signin", patch(ctr.Signin, "", false)).Methods("POST")
	api.Handle("/signin/google", patch(ctr.GoogleSignin, "", false)).Methods("POST")
	api.Handle("/signin/facebook", patch(ctr.FbSignin, "", false)).Methods("POST")
	api.Handle("/renew", patch(ctr.RenewToken, "", true)).Methods("POST")
	api.Handle("/get-org/{id:[0-9]+}", patch(ctr.GetOrg, "", false)).Methods("GET")
	api.Handle("/create-org", patch(ctr.CreateOrg, "", false)).Methods("POST")

	api.Handle("/me", patch(ctr.GetMeDetails, "", true)).Methods("GET")
	api.Handle("/user/reset", patch(ctr.SendResetToken, "", false)).Methods("POST")
	api.Handle("/user/updateprofile", patch(ctr.UpdateProfile, "", true)).Methods("POST")
	api.Handle("/user/changepassword/{token:[a-z0-9]+}", patch(ctr.ChangePassword, "", false)).Methods("POST")
	api.Handle("/user/authorize", patch(ctr.Authorize, "", true)).Methods("POST")

	// Static file server
	fileServer := http.FileServer(http.Dir("./profile_pics/"))
	r.PathPrefix("/avatar/").Handler(http.StripPrefix("/avatar/", fileServer))
	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)
	handler := cors.AllowAll().Handler(r)
	// r.Handle("/avatar/", http.StripPrefix("/avatar", fileServer))

	return http.StripPrefix(settings.BaseURL, handler), nil

}
