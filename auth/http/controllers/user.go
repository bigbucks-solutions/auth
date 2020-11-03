package controllers

import (
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/settings"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type SentTokenBody struct {
	Email string
}

type ResetPasswordBody struct {
	Token    string
	Password string
	Email    string
}

func SentResetToken(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	// posts = append(posts, Post{ID: "1", Title: "My first post", Body: "This is the content of my first post"})
	var rst SentTokenBody
	json.NewDecoder(r.Body).Decode(&rst)
	var usr models.User
	models.Dbcon.Preload("Profile").Find(&usr, &models.User{Username: rst.Email})
	usr.GenerateResetToken()
	// vars := mux.Vars(r)
	// id, _ := strconv.Atoi(vars["id"])
	// org, _, _ := models.GetOrganization(id)
	// w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"msg": "Password reset token sent to registered email"})
	return 0, nil
}

func ChangePassword(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	fmt.Println("testes")
	var body ResetPasswordBody
	err := json.NewDecoder(r.Body).Decode(&body)
	vars := mux.Vars(r)
	body.Token = vars["token"]
	var usr models.User
	err = models.Dbcon.Preload("ForgotPassword").First(&usr, "username = ?", body.Email).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return http.StatusBadRequest, err
	}
	num, err := usr.ChangePassword(body.Token, body.Password)
	if err != nil {
		return num, err
	}

	// code, err := models.CreateOrganization(&org)
	// posts = append(posts, Post{ID: "1", Title: "My first post", Body: "This is the content of my first post"})
	// w.Header().Set("Content-Type", "application/json")
	// json.NewEncoder(w).Encode(err)
	return 0, nil
}
