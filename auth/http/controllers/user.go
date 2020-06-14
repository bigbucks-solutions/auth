package controllers

import (
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/settings"
	"encoding/json"
	"net/http"
)

type Post struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

var posts []Post

func GetPosts(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error) {
	posts = append(posts, Post{ID: "1", Title: "My first post", Body: "This is the content of my first post"})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
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
