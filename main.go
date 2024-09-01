package main

import (
	"emlserver/config"
	"emlserver/database"
	"emlserver/discord"
	"emlserver/security"
	"emlserver/webserver"

	_ "github.com/lib/pq"
)

func main() {
	err := config.LoadConfig("server.cfg")
	if err != nil {
		panic(err)
	}

	security.InitSecurity()
	database.ConnectDatabase()
	discord.BeginClient()
	webserver.InitializeWebserver()
}
