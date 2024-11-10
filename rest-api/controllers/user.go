package controllers

import (
	. "bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/rest-api/controllers/types"
	"bigbucks/solution/auth/settings"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type RequestPasswordResetToken struct {
	Email string `json:"email" example:"example@example.com"`
}
type ResetPassword struct {
	Token    string `json:"-"`
	Password string
	Email    string
}

type UpdateProfileBody struct {
	File *multipart.FileHeader `json:"file"`
}

// PasswordReset godoc
// @Summary      Send the password reset token
// @Description  Get password reset token to email
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body  RequestPasswordResetToken  true  "request body"
// @Success      200  {object}  types.SimpleResponse  "return"
// @Failure      400  ""
// @Failure      404  ""
// @Failure      500  ""
// @Router       /user/reset [post]
func SendResetToken(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	var requestBody RequestPasswordResetToken
	json.NewDecoder(r.Body).Decode(&requestBody)
	var usr models.User
	models.Dbcon.Preload("Profile").Find(&usr, &models.User{Username: requestBody.Email})
	usr.GenerateResetToken()
	Logger.Debugln("Sending Reset Token..")
	json.NewEncoder(w).Encode(&types.SimpleResponse{Message: "Password reset token sent to registered email"})
	return 0, nil
}

// ChangePassword godoc
// @Summary      Reset the password with the password reset token sent
// @Description  Reset the password with the password reset token sent to email
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        token  path  string  true  "token"
// @Param        request  body  ResetPassword  true  "request body"
// @Success      200  "return"
// @Failure      400  ""
// @Failure      404  ""
// @Failure      500  ""
// @Router       /user/changepassword/{token} [post]
func ChangePassword(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	var body ResetPassword
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
	return 0, nil
}

// UpdateProfile godoc
// @Summary      Update User profile details
// @Description  Update user profile details
// @Tags         auth
// @Param 		 X-Auth header string true "Authorization"
// @Security 	 JWTAuth
// @Accept       multipart/form-data
// @Produce      json
// @Param        request  formData  file  true  "formData"
// @Success      200  ""
// @Failure      400  ""
// @Failure      404  ""
// @Failure      500  ""
// @Router       /user/updateprofile [post]
func UpdateProfile(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	parseErr := r.ParseMultipartForm(32 << 20) // maxMemory 32MB
	if parseErr != nil {
		return http.StatusBadRequest, parseErr
	}
	user, err := ctx.GetCurrentUserModel()
	num, err := user.UpdateUserProfile(r.Form, r.MultipartForm.File["file"])
	return num, err
}

// GetProfileDetails godoc
// @Summary      Get logged in user profile information
// @Tags         auth
// @Accept       json
// @Param 		 X-Auth header string true "Authorization"
// @Security 	 JWTAuth
// @Produce      json
// @Success      200  {object}  types.UserInfo  ""
// @Failure      400  ""
// @Failure      500  ""
// @Router       /me [get]
func GetMeDetails(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	user, err := ctx.GetCurrentUserModel()
	if err != nil {
		return http.StatusNotFound, err
	}
	json.NewEncoder(w).Encode(user)
	return 0, nil
}

// Signup godoc
// @Summary      Register a new user
// @Description  Create a new user account
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body  types.SignupRequestBody  true  "User signup details"
// @Router       /signup [post]
func Signup(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	var signupRequest types.SignupRequestBody
	if err := json.NewDecoder(r.Body).Decode(&signupRequest); err != nil {
		return http.StatusBadRequest, err
	}

	user := models.User{
		Username: signupRequest.Email,
		Password: signupRequest.Password,
		Profile: models.Profile{
			FirstName: signupRequest.FirstName,
			LastName:  signupRequest.LastName,
			Email:     signupRequest.Email,
		},
	}

	if err := models.Dbcon.Create(&user).Error; err != nil {
		return http.StatusInternalServerError, err
	}

	json.NewEncoder(w).Encode(&types.SimpleResponse{
		Message: "User registered successfully",
	})
	return 0, nil
}
