package gist

import (
	"time"
)

// PostHistory database table representation, linked with the Post.
type PostHistory struct {
	PostID    int       `json:"post_id"`
	CreatedAt time.Time `json:"created_at"`
	Body      string    `json:"body"`
}

// Post database table representation.
type Post struct {
	PostID    int           `json:"post_id"`
	UserID    int           `json:"user_id"` // the author(user) id
	CreatedAt time.Time     `json:"created_at"`
	Body      string        `json:"body"`
	History   []PostHistory // the edits
}

type postUtils struct{}

// NewHistory returns a new PostHistory.
func (p postUtils) NewHistory(postID int, createdAt time.Time, body string) PostHistory {
	return PostHistory{
		PostID:    postID,
		CreatedAt: createdAt,
		Body:      body,
	}
}

// New returns a new Post.
func (p postUtils) New(id int, userID int, createdAt time.Time, body string, history []PostHistory) Post {
	return Post{
		PostID:    id,
		UserID:    userID,
		CreatedAt: createdAt,
		Body:      body,
		History:   history,
	}
}
