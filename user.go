package gist

import (
	"fmt"
	"time"
)

// User represents the user object in database
// linked to a github user profile.
// Note: We will not use the github oauth2 api.
//       Why? Newcomers may afraid of giving accepting their github acc permission to use this site.
//       No app can store their password and things like these, but newcomers afraid, I had an experience of this before.
//       So, for now we accept only new created users linked with the site's database itself.
//       At the future, when people will trust the site I'll add authentication from github as an optional login method too.
type User struct {
	UserID      int       `json:"user_id"` // int should be enough, I don't expect billion of users xD
	Username    string    `json:"username"`
	Password    string    `json:"-"`
	Mail        string    `json:"-"`
	CreatedAt   time.Time `json:"-"`
	LastLoginAt time.Time `json:"-"`
	Avatar      string    `json:"avatar"`
}

type userUtils struct {
}

const githubAvatarScheme = "https://github.com/%s.png"

// GetAvatarURI returns a new avatar uri based on the github user's username.
func (u userUtils) GetAvatarURI(username string) string {
	return fmt.Sprintf(githubAvatarScheme, username)
}

// let return the User as it's and remove pass and mail from the encoding
// func (u userUtils) Export(user User) map[string]interface{} {
// 	return map[string]interface{}{
// 		"id":       user.ID, // used in client side to do the url mapping
// 		"username": user.Username,
// 		"avatar":   user.Avatar,
// 	}
// }

// New returns a new User.
func (u userUtils) New(id int, username, password, mail string,
	lastLoginAt, createdAt time.Time) User {
	return User{
		UserID:      id,
		Username:    username,
		Password:    password,
		Mail:        mail,
		CreatedAt:   createdAt,
		LastLoginAt: lastLoginAt,
		Avatar:      u.GetAvatarURI(username),
	}
}
