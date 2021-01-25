package controllers

import (
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/settings"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type Post struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

var posts []Post

func GetOrg(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	// posts = append(posts, Post{ID: "1", Title: "My first post", Body: "This is the content of my first post"})
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])
	org, _, _ := models.GetOrganization(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(org)
	return 0, nil
}

func CreateOrg(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	var org models.Organization
	err := json.NewDecoder(r.Body).Decode(&org)
	if err != nil {
		return http.StatusBadRequest, err
	}

	code, err := models.CreateOrganization(&org)
	// posts = append(posts, Post{ID: "1", Title: "My first post", Body: "This is the content of my first post"})
	// w.Header().Set("Content-Type", "application/json")
	// json.NewEncoder(w).Encode(err)
	return code, err
}
