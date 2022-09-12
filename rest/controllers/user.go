package controllers

import (
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/rest/controllers/types"
	"bigbucks/solution/auth/settings"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type ResetPasswordBody struct {
	Token    string
	Password string
	Email    string
}

// PasswordReset godoc
// @Summary      Send the password reset token
// @Description  Get password reset token to email
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body  controllers.SendResetToken.PasswordResetBody  true  "request body"
// @Success      200  {object}  types.SimpleResponse  "return"
// @Failure      400  ""
// @Failure      404  ""
// @Failure      500  ""
// @Router       /user/reset [post]
func SendResetToken(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	type PasswordResetBody struct {
		Email string `json:"email" example:"example@example.com"`
	}
	var response PasswordResetBody
	json.NewDecoder(r.Body).Decode(&response)
	var usr models.User
	models.Dbcon.Preload("Profile").Find(&usr, &models.User{Username: response.Email})
	usr.GenerateResetToken()
	log.Println("Sending Reset Token...")
	json.NewEncoder(w).Encode(&types.SimpleResponse{Message: "Password reset token sent to registered email"})
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

func UpdateProfile(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	parseErr := r.ParseMultipartForm(32 << 20) // maxMemory 32MB
	if parseErr != nil {
		// http.Error(w, "failed to parse multipart message", http.StatusBadRequest)
		return http.StatusBadRequest, parseErr
	}
	num, err := ctx.User.UpdateUserProfile(r.Form, r.MultipartForm.File["file"])

	// r.MultipartForm.Value[]

	// code, err := models.CreateOrganization(&org)
	// posts = append(posts, Post{ID: "1", Title: "My first post", Body: "This is the content of my first post"})
	// w.Header().Set("Content-Type", "application/json")
	// json.NewEncoder(w).Encode(err)
	return num, err
}

func GetMeDetails(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	log.Printf("called me")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(ctx.User)
	return 0, nil
}
