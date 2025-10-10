package controllers

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/request_context"
	"bigbucks/solution/auth/rest-api/controllers/types"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"slices"
	"strconv"

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

// SendResetToken godoc
//
//	@Summary		Send the password reset token
//	@Description	Get password reset token to email
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RequestPasswordResetToken	true	"request body"
//	@Success		200		{object}	types.SimpleResponse		"return message"
//	@Failure		400		""
//	@Failure		404		""
//	@Failure		500		""
//	@Router			/user/reset [post]
func SendResetToken(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var requestBody RequestPasswordResetToken
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		return http.StatusBadRequest, err
	}
	var usr models.User
	models.Dbcon.Preload("Profile").Find(&usr, &models.User{Username: requestBody.Email})
	_, err = usr.GenerateResetToken()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	loging.Logger.Debugln("Sending Reset Token..")
	err = json.NewEncoder(w).Encode(&types.SimpleResponse{Message: "Password reset token sent to registered email"})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// ChangePassword godoc
//
//	@Summary		Reset the password with the password reset token sent
//	@Description	Reset the password with the password reset token sent to email
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			token	path	string			true	"token"
//	@Param			request	body	ResetPassword	true	"request body"
//	@Success		200		""
//	@Failure		400		""
//	@Failure		404		""
//	@Failure		500		""
//	@Router			/user/changepassword/{token} [post]
func ChangePassword(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var body ResetPassword
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		return http.StatusBadRequest, err
	}
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
//
//	@Summary		Update User profile details
//	@Description	Update user profile details
//	@Tags			auth
//	@Param			X-Auth	header	string	true	"Authorization"
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			request	formData	file	true	"formData"
//	@Success		200		""
//	@Failure		400		""
//	@Failure		404		""
//	@Failure		500		""
//	@Router			/user/updateprofile [post]
func UpdateProfile(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	parseErr := r.ParseMultipartForm(32 << 20) // maxMemory 32MB
	if parseErr != nil {
		return http.StatusBadRequest, parseErr
	}
	user, err := ctx.GetCurrentUserModel()
	if err != nil {
		return http.StatusBadRequest, err
	}
	num, err := user.UpdateUserProfile(r.Form, r.MultipartForm.File["file"])
	return num, err
}

// GetMeDetails godoc
//
//	@Summary	Get logged in user profile information
//	@Tags		auth
//	@Accept		json
//	@Param		X-Auth	header	string	true	"Authorization"
//	@Produce	json
//	@Success	200	{object}	types.UserInfo	"User details"
//	@Failure	400	""
//	@Failure	500	""
//	@Router		/me [get]
func GetMeDetails(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	user, err := ctx.GetCurrentUserModel()
	if err != nil {
		return http.StatusNotFound, err
	}
	err = json.NewEncoder(w).Encode(user)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// Signup godoc
//
//	@Summary		Register a new user
//	@Description	Create a new user account
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.SignupRequestBody	true	"User signup details"
//
//	@Success		200		{object}	types.SimpleResponse	"Success message"
//	@Failure		400		{object}	error					"Bad request"
//	@Failure		404		{object}	error					"Not found"
//	@Failure		500		{object}	error					"Internal server error"
//
//	@Router			/signup [post]
func Signup(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
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

	err := json.NewEncoder(w).Encode(&types.SimpleResponse{
		Message: "User registered successfully",
	})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// GetUsers godoc
//
//	@Summary		Lists the users
//	@Description	Lists the users for an organization
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			X-Auth	header	string	true	"Authorization"
//	@Security		JWTAuth
//	@Param			page		query		int		false	"Page number"	default(1)
//	@Param			page_size	query		int		false	"Page size"		default(10)
//	@Param			role_id		query		string	false	"Filter by role name"
//	@Param			org_id		query		int		false	"Filter by organization ID"
//	@Success		200			{object}	actions.UserListResponse
//	@Security		JWTAuth
//	@Router			/users [get]
func GetUsers(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	searchPrefix := r.URL.Query().Get("search_prefix")
	roleID := r.URL.Query().Get("role_id")
	userStatus := constants.UserStatus(r.URL.Query().Get("status"))
	var userStatusPtr *constants.UserStatus
	if !slices.Contains(constants.UserStatuses, userStatus) {
		userStatusPtr = nil
	} else {
		userStatusPtr = &userStatus
	}

	users, code, err := actions.ListUsersForOrg(ctx.CurrentOrgID, page, pageSize, userStatusPtr, &roleID, &searchPrefix, ctx.SessionStore)
	if err != nil {
		return code, err
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(users)
	if err != nil {
		loging.Logger.Error("Error encoding users", err)
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// ActivateUser godoc
//
//	@Summary		Activate a user
//	@Description	Activate a user in the organization
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			X-Auth	header	string	true	"Authorization"
//	@Security		JWTAuth
//	@Param			user_id	path	string	true	"User ID"
//	@Success		200		{object}	types.SimpleResponse	"Success message"
//	@Failure		400		{object}	error					"Bad request"
//	@Failure		404		{object}	error					"User not found"
//	@Failure		500		{object}	error					"Internal server error"
//	@Router			/users/{user_id}/activate [put]
func ActivateUser(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	params := actions.ActivateUserParams{
		UserID: userID,
		OrgID:  ctx.CurrentOrgID,
	}

	code, err := actions.ActivateUser(params)
	if err != nil {
		return code, err
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&types.SimpleResponse{
		Message: "User activated successfully",
	})
	if err != nil {
		loging.Logger.Error("Error encoding activate user response", err)
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// DeactivateUser godoc
//
//	@Summary		Deactivate a user
//	@Description	Deactivate a user in the organization
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			X-Auth	header	string	true	"Authorization"
//	@Security		JWTAuth
//	@Param			user_id	path	string	true	"User ID"
//	@Success		200		{object}	types.SimpleResponse	"Success message"
//	@Failure		400		{object}	error					"Bad request"
//	@Failure		404		{object}	error					"User not found"
//	@Failure		500		{object}	error					"Internal server error"
//	@Router			/users/{user_id}/deactivate [put]
func DeactivateUser(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	params := actions.DeactivateUserParams{
		UserID: userID,
		OrgID:  ctx.CurrentOrgID,
	}

	code, err := actions.DeactivateUser(params)
	if err != nil {
		return code, err
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&types.SimpleResponse{
		Message: "User deactivated successfully",
	})
	if err != nil {
		loging.Logger.Error("Error encoding deactivate user response", err)
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
