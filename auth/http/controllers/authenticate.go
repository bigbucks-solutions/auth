package controllers

import (
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/settings"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/dgrijalva/jwt-go/request"
)

type userInfo struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

type AuthToken struct {
	User userInfo `json:"user"`
	jwt.StandardClaims
}

// Struct for parsing json body login credentials
type jsonCred struct {
	Password  string `json:"password"`
	Username  string `json:"username"`
	ReCaptcha string `json:"recaptcha"`
}

type Extractor []string

func (e Extractor) ExtractToken(r *http.Request) (string, error) {
	token, _ := request.HeaderExtractor{"X-Auth"}.ExtractToken(r)
	if token != "" && strings.Count(token, ".") == 2 {
		return token, nil
	}
	token = r.URL.Query().Get("auth")
	if token != "" && strings.Count(token, ".") == 2 {
		return token, nil
	}
	return "", request.ErrNoTokenInRequest
}

func Signin(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	fmt.Println("Logged in..")
	var cred jsonCred
	err := json.NewDecoder(r.Body).Decode(&cred)
	if err != nil {
		return http.StatusBadRequest, err
	}
	success, user := models.Authenticate(cred.Username, cred.Password)
	if !success {
		return http.StatusUnauthorized, nil
	}
	return printToken(w, r, &user, &ctx.Settings)
}

func RenewToken(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	return printToken(w, r, &ctx.User, &ctx.Settings)
}

func printToken(w http.ResponseWriter, r *http.Request, user *models.User, settings *settings.Settings) (int, error) {
	claims := &AuthToken{
		User: userInfo{
			Username: user.Username,
			Roles:    []string{"", ""},
		},
		StandardClaims: jwt.StandardClaims{
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(time.Hour * 2).Unix(),
			Issuer:    "BigBucks",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(settings.SecretKey))
	if err != nil {
		return http.StatusInternalServerError, err
	}
	w.WriteHeader(http.StatusAccepted)
	w.Header().Set("Content-Type", "cty")
	w.Write([]byte(signed))
	return 0, nil
}
