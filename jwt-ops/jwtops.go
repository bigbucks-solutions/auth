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

	// Extract from header Bearer AUthorization
	token, _ = request.AuthorizationHeaderExtractor.ExtractToken(r)
	if token != "" && strings.Count(token, ".") == 2 {
		Logger.Debugln("Token from header", token)
		return token, nil

	}

	token = r.URL.Query().Get("auth")
	if token != "" && strings.Count(token, ".") == 2 {
		return token, nil
	}
	// Extract from cookie
	cookie, err := r.Cookie("auth")
	if err == nil {
		return cookie.Value, nil
	}
	return "", request.ErrNoTokenInRequest
}

func SignJWT(user *models.User, sessionId string) (signed string, err error) {
	var userOrgRole []settings.UserOrgRole
	for _, role := range user.Roles {
		userOrgRole = append(userOrgRole, settings.UserOrgRole{
			Role:  role.Name,
			OrgID: role.OrgID,
		})
	}
	claims := &settings.AuthToken{
		User: settings.UserInfo{
			Username: user.Username,
			Roles:    userOrgRole,
		},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * 60)),
			Issuer:    "BigBucks Auth",
			ID:        sessionId,
		},
	}
	signingMethod := jwt.GetSigningMethod(settings.Current.Alg)
	token := jwt.NewWithClaims(signingMethod, claims)
	signingKey, err := jwt.ParseECPrivateKeyFromPEM(settings.SingingKey)
	if err != nil {
		Logger.Error("Error parsing signing key", zap.Error(err))
		return
	}

	signed, err = token.SignedString(signingKey)
	if err != nil {
		Logger.Error("Error signing token", zap.Error(err))
		return
	}
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
		token, err = request.ParseFromRequest(obj.(*http.Request), &Extractor{}, keyFunc, request.WithClaims(&claims))
	case string:
		token, err = jwt.ParseWithClaims(obj.(string), &claims, keyFunc)
	}

	if err != nil {
		Logger.Error("Error verifying token", zap.Error(err))

	}
	if token == nil {
		Logger.Warn("Token is empty")
		return

	}

	return
}
