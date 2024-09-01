package main

import (
	"database/sql"
	"emlserver/database"
	"emlserver/security"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type LegacyUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Bio      string `json:"bio"`
	Token    string `json:"token"`
	Id       uint64 `json:"id"`
}

type LegacyCommentBody struct {
	PageId   string          `json:"pageid"`
	Comments []LegacyComment `json:"comments"`
}

type LegacyModStats struct {
	ModID     int   `json:"mod"`
	Downloads int   `json:"downloads"`
	Likers    []int `json:"likers"`
}

type LegacyMod struct {
	Id          string `json:"id"`
	Author      int    `json:"author"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Game        string `json:"game"`
	Platform    string `json:"platform"`
	Video       string `json:"youtubevideo"`
	Published   bool   `json:"published"`
	Version     int    `json:"update"`
}

type LegacyComment struct {
	User      int    `json:"user"`
	CommentID string `json:"id"`
	Comment   string `json:"comment"`
}

type UserPass struct {
	ID       string `json:"id"`
	Password string `json:"password"`
}

func ReadLegacyDatabase(path string) []byte {
	content, err := os.ReadFile(path)
	if err != nil {
		println("could not read ", path)
		return []byte{}
	}
	formatted_content := fmt.Sprint("[", string(content))
	formatted_content = fmt.Sprint(strings.Replace(formatted_content, "\n", ",", -1)[:len(formatted_content)-1], "]")

	return []byte(formatted_content)
}

func Migrate(db *sql.DB) {
	LegacyModStatsDatabaseMigration(db)
}

func LegacyUserDatabaseMigration(db *sql.DB) {

	formatted_data := ReadLegacyDatabase("legacy/users.db")

	data := []LegacyUser{}

	err := json.Unmarshal(formatted_data, &data)

	if err != nil {
		log.Panic(err)
	}

	added_users := []uint64{}

	for i, e := range data {
		println(i, ": Processing...")

		already_added := false

		for _, a := range added_users {
			if a == e.Id {
				already_added = true
				break
			}
		}

		if already_added {
			continue
		}

		added_users = append(added_users, e.Id)
		_, err := db.Exec("INSERT INTO users VALUES ($1, $2, $3, $4, $5)", e.Id, strings.Replace(e.Username, " ", "_", -1), e.Password, e.Token, e.Bio)
		if err != nil {
			log.Panic(err)
		}

	}
}

func LegacyModStatsDatabaseMigration(db *sql.DB) {
	formatted_data := ReadLegacyDatabase("legacy/modstats.db")
	data := []LegacyModStats{}
	err := json.Unmarshal(formatted_data, &data)
	if err != nil {
		log.Panic(err)
	}
	index := 0
	for _, e := range data {
		for _, l := range e.Likers {
			index += 1
			timestamp := time.Now().Unix() + int64(index)
			_, err := db.Exec("INSERT INTO likes VALUES ($1, $2, $3)", e.ModID, fmt.Sprint(l), timestamp)
			if err != nil {
				log.Panic(err)
			}
		}

		_, err := db.Exec("UPDATE mods SET downloads=$1 WHERE id=$2", e.Downloads, e.ModID)
		if err != nil {
			log.Panic(err)
		}
	}
}

func LegacyModDatabaseMigration(db *sql.DB) {
	formatted_data := ReadLegacyDatabase("legacy/mods.db")

	data := []LegacyMod{}

	err := json.Unmarshal(formatted_data, &data)

	if err != nil {
		log.Panic(err)
	}

	for _, e := range data {
		p := 0
		if e.Published {
			p = 1
		} else {
			p = 0
		}
		_, err := db.Exec("INSERT INTO mods VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)", e.Id, e.Name, e.Description, e.Game, e.Platform, e.Video, 0, e.Version, fmt.Sprint(e.Author), p)
		if err != nil {
			log.Panic(err)
		}
	}

}

func UpdatePassword() {
	content, err := os.ReadFile("pass.json")
	if err != nil {
		panic(err)
	}

	data := []UserPass{}

	err = json.Unmarshal(content, &data)

	if err != nil {
		panic(err)
	}

	for _, e := range data {
		hashed := security.Hash(e.Password)
		_, err := database.Database.Exec("UPDATE users SET password=$1 WHERE id=$2", hashed, e.ID)
		if err != nil {
			panic(err)
		}
	}
}

func LegacyCommentDatabaseMigration(db *sql.DB) {
	formatted_data := ReadLegacyDatabase("legacy/comments.db")
	data := []LegacyCommentBody{}

	err := json.Unmarshal(formatted_data, &data)

	if err != nil {
		log.Panic(err)
	}

	print(len(data))

	for _, body := range data {
		println(body.PageId, ": Processing...")
		for _, comment := range body.Comments {

			_, err := db.Exec("INSERT INTO comments VALUES ($1, $2, $3, $4)", fmt.Sprint(body.PageId), comment.Comment, comment.CommentID, fmt.Sprint(comment.User))

			if err != nil {
				log.Panic(err)
			}
		}
	}
}
