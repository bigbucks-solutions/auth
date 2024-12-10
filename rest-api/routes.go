package rest

import (
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/request_context"
	ctr "bigbucks/solution/auth/rest-api/controllers" //Load all controllers methods by deafult
	"bigbucks/solution/auth/settings"
	"net/http"

	_ "bigbucks/solution/auth/docs"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	httpSwagger "github.com/swaggo/http-swagger"
)

type handleFunc func(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error)

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
func NewHandler(settings *settings.Settings, perm_cache *permission_cache.PermissionCache) (http.Handler, error) {
	settings.Clean()

	r := mux.NewRouter()

	makeHandler := func(fn handleFunc, opts ...HandlerOption) http.Handler {
		config := &handlerConfig{
			prefix: "",
			auth:   false,
		}

		for _, opt := range opts {
			opt(config)
		}

		return handle(fn, config, settings, perm_cache)
	}
	//cors

	api := r.PathPrefix("/api/v1").Subrouter()
	api.Handle("/signin", makeHandler(ctr.Signin)).Methods("POST")
	api.Handle("/signup", makeHandler(ctr.Signup)).Methods("POST")
	api.Handle("/signin/google", makeHandler(ctr.GoogleSignin)).Methods("POST")
	api.Handle("/signin/facebook", makeHandler(ctr.FbSignin)).Methods("POST")
	api.Handle("/renew", makeHandler(ctr.RenewToken, WithAuth(true))).Methods("POST")
	api.Handle("/get-org/{id:[0-9]+}", makeHandler(ctr.GetOrg)).Methods("GET")
	api.Handle("/create-org", makeHandler(ctr.CreateOrg)).Methods("POST")

	api.Handle("/me", makeHandler(ctr.GetMeDetails, WithAuth(true))).Methods("GET")
	api.Handle("/user/reset", makeHandler(ctr.SendResetToken)).Methods("POST")
	api.Handle("/user/updateprofile", makeHandler(ctr.UpdateProfile, WithAuth(true))).Methods("POST")
	api.Handle("/user/changepassword/{token:[a-z0-9]+}", makeHandler(ctr.ChangePassword)).Methods("POST")
	api.Handle("/user/authorize", makeHandler(ctr.Authorize, WithAuth(true))).Methods("POST")

	api.Handle("/roles", makeHandler(ctr.ListRoles, WithAuth(true))).Methods("GET")
	api.Handle("/roles", makeHandler(ctr.CreateRole, WithAuth(true))).Methods("POST")
	api.Handle("/permissions", makeHandler(ctr.CreatePermission, WithAuth(true))).Methods("POST")
	api.Handle("/roles/bind-permission", makeHandler(ctr.BindPermissionToRole, WithAuth(true))).Methods("POST")

	// Static file server
	fileServer := http.FileServer(http.Dir("./profile_pics/"))
	r.PathPrefix("/avatar/").Handler(http.StripPrefix("/avatar/", fileServer))
	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)
	// r.Handle("/avatar/", http.StripPrefix("/avatar", fileServer))
	handler := cors.AllowAll().Handler(r)

	return http.StripPrefix(settings.BaseURL, handler), nil

}
