package jwtops

import (
	. "bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/settings"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/golang-jwt/jwt/v4/request"
	"go.uber.org/zap"
)

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

func SignJWT(user *models.User) (signed string, err error) {
	claims := &settings.AuthToken{
		User: settings.UserInfo{
			Username: user.Username,
			Roles:    []string{"", ""},
		},
		StandardClaims: jwt.StandardClaims{
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(time.Minute * 20).Unix(),
			Issuer:    "BigBucks Auth",
		},
	}
	signingMethod := jwt.GetSigningMethod(settings.Current.Alg)
	token := jwt.NewWithClaims(signingMethod, claims)
	signingKey, err := jwt.ParseECPrivateKeyFromPEM(settings.SingingKey)
	signed, err = token.SignedString(signingKey)
	return
}

func VerifyJWT(obj interface{}) (claims settings.AuthToken, token *jwt.Token, err error) {
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		key, err := jwt.ParseECPublicKeyFromPEM(settings.VerifyingKey)
		return key, err
	}
	switch v := obj.(type) {
	default:
		Logger.Error("unexpected type %T", v)
		panic(v)
	case *http.Request:
		token, err = request.ParseFromRequestWithClaims(obj.(*http.Request), &Extractor{}, &claims, keyFunc)
	case string:
		token, err = jwt.ParseWithClaims(obj.(string), &claims, keyFunc)
	}

	if err != nil || !token.Valid {
		Logger.Debugln(zap.Bool("Token Validity", token.Valid))
	}

	return
}
