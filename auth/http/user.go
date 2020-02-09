package http

import (
	"net/http"
	"encoding/json"
  )

type Post struct {
	ID string `json:"id"`
	Title string `json:"title"`
	Body string `json:"body"`
  }
var posts []Post



func getPosts(w http.ResponseWriter, r *http.Request) (int, error){
	posts = append(posts, Post{ID: "1", Title: "My first post", Body:      "This is the content of my first post"})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
	return 0, nil
  }