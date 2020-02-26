package controllers

import (
	"net/http"
	"encoding/json"
	"bigbucks/solution/auth/settings"
  )

type Post struct {
	ID string `json:"id"`
	Title string `json:"title"`
	Body string `json:"body"`
  }
var posts []Post



func GetPosts(w http.ResponseWriter, r *http.Request, ctx *settings.Context) (int, error){
	posts = append(posts, Post{ID: "1", Title: "My first post", Body:      "This is the content of my first post"})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
	return 0, nil
  }