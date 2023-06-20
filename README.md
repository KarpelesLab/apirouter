[![GoDoc](https://godoc.org/github.com/KarpelesLab/apirouter?status.svg)](https://godoc.org/github.com/KarpelesLab/apirouter)

# API router

# Returning errors

When returning an error, it is good practice to use the Error object.

# User

Setting up a request hook that will check the authorization header and
call Context.SetUser allows passing an object to other methods.
