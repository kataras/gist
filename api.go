package gist

func registerAPI() {
	users := App.Party("/users")
	{
		users.Post("/", createUserHandler)
		users.Get("/:id/posts", getPostsByUserID)
	}

	posts := App.Party("/posts")
	{
		posts.Post("/", createPostHandler)
		posts.Get("/:id", getPostByIDHandler)
	}
}
