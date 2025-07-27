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
	config.LoadConfig("server.cfg")
	security.InitSecurity()
	database.ConnectDatabase()
	discord.BeginClient()
	webserver.InitializeWebserver()
}
