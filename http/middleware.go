package http

import (
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/settings"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	ctr "bigbucks/solution/auth/http/controllers" //Load all controllers methods by deafult

	"github.com/dgrijalva/jwt-go"
	"github.com/dgrijalva/jwt-go/request"
	"gorm.io/gorm"
)

func Authenticate(w http.ResponseWriter, r *http.Request, settings *settings.Settings) (bool, models.User, error) {
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		log.Println(settings.SecretKey)
		return []byte(settings.SecretKey), nil
	}

	var tk ctr.AuthToken
	token, err := request.ParseFromRequestWithClaims(r, &ctr.Extractor{}, &tk, keyFunc)
	// log.Printf(token.Raw)

	if err != nil || !token.Valid {
		log.Println("failed", err)
		return false, models.User{}, nil
		// log.Println("failed")
	}

	expired := !tk.VerifyExpiresAt(time.Now().Add(time.Hour).Unix(), true)
	// updated := d.store.Users.LastUpdate(tk.User.ID) > tk.IssuedAt

	if expired {
		w.Header().Add("X-Renew-Token", "true")
	}

	var user models.User
	if err := models.Dbcon.Where("username = ?", tk.User.Username).First(&user).Error; gorm.ErrRecordNotFound == err {
		log.Println("failed", err)
		return false, models.User{}, nil

	}
	// if err != nil {
	// 	return false, http.StatusInternalServerError, err
	// }
	return true, user, nil
}

func JSONError(w http.ResponseWriter, err interface{}, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(err)
}

func handle(fn handleFunc, prefix string, auth bool, setting *settings.Settings) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := &settings.Context{}
		ctx.Settings = *setting
		if auth {
			success, user, _ := Authenticate(w, r, setting)
			log.Println(success, user.Username)
			if !success {
				http.Error(w, strconv.Itoa(http.StatusForbidden)+" "+http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			ctx.User = user
		}
		status, err := fn(w, r, ctx)

		if status != 0 {
			// txt := http.StatusText(status)
			JSONError(w, err, status)
			return
		}
	})

	return http.StripPrefix(prefix, handler)
}
