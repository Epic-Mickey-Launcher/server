package database

import (
	"bytes"
	"database/sql"
	"emlserver/config"
	"emlserver/security"
	"emlserver/structs"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/google/uuid"
)

var Database *sql.DB

func GetMod(modid string) (structs.Mod, error) {
	modResult := structs.Mod{}

	row := Database.QueryRow("SELECT id, name, description, game, platform, youtube, version, author, published, downloads, likes, repositoryurl FROM mods WHERE id=$1", modid)

	err := row.Scan(&modResult.ID, &modResult.Name, &modResult.Description,
		&modResult.Game, &modResult.Platform, &modResult.Video,
		&modResult.Version, &modResult.Author, &modResult.Published,
		&modResult.Downloads, &modResult.CachedLikes, &modResult.RepositoryUrl)

	return modResult, err
}

func GetLikes(pageID string) (int, error) {
	query := Database.QueryRow("SELECT COUNT(*) FROM likes WHERE pageid=$1", pageID)
	var count int
	query.Scan(&count)
	return count, nil
}

func GetComment() {
}

type Pass struct {
	Id       string `json:"id"`
	Password string `json:"password"`
}

func ConnectDatabase() {
	dbURL := config.LoadedConfig["DBURL"]
	_db, err := sql.Open("postgres", dbURL)
	if err != nil {
		println("Error: Could not open Database: ", err)
	}
	if err = _db.Ping(); err != nil {
		println("Error: Could not ping Database: ", err)
	}

	println("Connection successful.")

	Database = _db
}

// User Management

func LikePage(userID string, pageID string) (structs.ResponseLiked, error) {
	err := UserLikedPage(userID, pageID)

	res := structs.ResponseLiked{
		Liked: err != nil,
	}

	if err != nil {

		ID := security.GenerateID()
		_, err = Database.Exec("INSERT INTO likes (author, pageid, likeid) VALUES ( $1, $2, $3 )", userID, pageID, ID)
		if err != nil {
			return structs.ResponseLiked{}, err
		}
	} else {
		_, err = Database.Exec("DELETE FROM likes WHERE pageid=$1 AND author=$2", pageID, userID)
		if err != nil {
			return structs.ResponseLiked{}, err
		}
	}

	return res, nil
}

func UserLikedPage(userID string, pageID string) error {
	row := Database.QueryRow("SELECT likeid FROM likes WHERE author=$1 AND pageid=$2", userID, pageID)
	var id string
	err := row.Scan(&id)
	return err
}

func GetUser(userid string) (structs.User, error) {
	userResult := structs.User{}

	row := Database.QueryRow("SELECT id, username, password, token, bio, email, emailhash FROM users WHERE id=$1", userid)

	err := row.Scan(&userResult.ID, &userResult.Username, &userResult.Password, &userResult.Token, &userResult.Bio, &userResult.Email, &userResult.EmailHash)

	return userResult, err
}

func DeleteUser(userid string) error {
	_, err := Database.Exec("DELETE FROM users WHERE id=$1", userid)
	return err
}

func CreateUser() (string, error) {
	id := security.GenerateID()
	_, err := Database.Exec("INSERT INTO users (id) VALUES ($1)", id)
	return id, err
}

func SetUsername(userid string, username string) int {
	_, err := Database.Exec("UPDATE users SET username=$1 WHERE id=$2", username, userid)
	if err != nil {
		println("Could not change username to", username)
		return -1
	}
	return 0
}

func InitEmail(userid string) error {
	_, err := Database.Exec("UPDATE users SET email='', emailhash='' WHERE id=$1", userid)
	return err
}

func SetBio(userid string, bio string) int {
	_, err := Database.Exec("UPDATE users SET bio=$1 WHERE id=$2", bio, userid)
	if err != nil {
		println("Could not change bio to", bio)
		return -1
	}
	return 0
}

func GenerateUserToken(userid string) (string, error) {
	token := uuid.New().String()
	_, err := Database.Exec("UPDATE users SET token=$1 WHERE id=$2", token, userid)
	if err != nil {
		println("Failed to generate user token.")
		return "", err
	}
	return token, nil
}

func SetPassword(userid string, password string) error {
	encryptedPassword := security.PassHash(password)
	_, err := Database.Exec("UPDATE users SET password=$1 WHERE id=$2", encryptedPassword, userid)
	if err != nil {
		println("Could not change user password for: ", userid)
		return errors.New("could not change password")
	}
	return nil
}

func ChangeAllPasswords() {
	content, err := os.ReadFile("pass.json")
	if err != nil {
		panic(err)
	}

	var data []structs.RequestRegisterAccount
	err = json.NewDecoder(bytes.NewReader(content)).Decode(&data)
	if err != nil {
		panic(err)
	}

	var count int

	for _, e := range data {
		println("changing password for " + fmt.Sprint(e.ID) + "(" + fmt.Sprint(count) + ")")
		SetPassword(fmt.Sprint(e.ID), e.Password)
		count++
	}
}

// Mod Management

const ModQueryLimit = 8

const (
	OrderNew             = 0
	OrderOld             = 1
	OrderLiked           = 2
	OrderLeastLiked      = 3
	OrderDownloaded      = 4
	OrderLeastDownloaded = 5
)

func QueryMods(modQuery structs.RequestModQuery) ([]string, int, error) {
	row := Database.QueryRow("SELECT id FROM users WHERE token=$1", modQuery.Token)

	var userID string

	err := row.Scan(&userID)
	if err != nil {
		userID = ""
	}

	querySize := 0

	if modQuery.QuerySize != 0 {
		querySize = modQuery.QuerySize
	} else {
		querySize = ModQueryLimit
	}

	offset := querySize * modQuery.PageIndex

	query := `SELECT id FROM mods `
	conditions := `WHERE ($1='' OR author=$1) 
                   AND   ($2='' OR name ILIKE  '%' || $2 || '%')
          	       AND   (published=B'1' OR (published=B'0' AND author=$3))
                   AND   (($4='' OR game=$4) AND ($5='' OR platform=$5))`

	var order string

	switch modQuery.Order {
	case 0: // Newest
		order = "GROUP BY id ORDER BY id DESC "
		break
	case 1: // Oldest
		order = "GROUP BY id ORDER BY id "
		break
	case 2: // Most Downloads
		order = "GROUP BY id ORDER BY downloads DESC "
		break
	case 3: // Least Downloads
		order = "GROUP BY id ORDER BY downloads ASC "
		break
	case 4: // Most Likes
		order = "GROUP BY id ORDER BY likes DESC "
		break
	case 5: // Least Likes
		order = "GROUP BY id ORDER BY likes ASC "
		break
	}

	limits := `LIMIT $6 OFFSET $7`

	query += conditions + order + limits

	rows, err := Database.Query(query, modQuery.AuthorID, modQuery.SearchQuery, userID, modQuery.Game, strings.ToLower(modQuery.Platform), querySize, offset)
	if err != nil {
		return nil, 0, err
	}

	defer rows.Close()

	var idArray []string

	for rows.Next() {
		var id string
		err = rows.Scan(&id)
		if err != nil {
			return nil, 0, err
		}

		idArray = append(idArray, id)
	}

	query = `SELECT COUNT(*) FROM mods ` + conditions

	row = Database.QueryRow(query, modQuery.AuthorID, modQuery.SearchQuery, userID, modQuery.Game, strings.ToLower(modQuery.Platform))
	var count int
	err = row.Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	pages := math.Round(float64(count / querySize))

	if float64(count)/float64(querySize) > pages {
		pages += 1
	}

	return idArray, int(pages), nil
}

func CreateMod(mod structs.Mod) error {
	var publish int
	if mod.Published {
		publish = 1
	} else {
		publish = 0
	}

	_, err := Database.Exec("INSERT INTO mods (id, name, description, game, platform, youtube, version, author, published, downloads, like, repositoryurl) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)", mod.ID, mod.Name, mod.Description, mod.Game, mod.Platform, mod.Video, 0, mod.Author, publish, 0, 1, mod.RepositoryUrl)
	if err != nil {
		println(err.Error())
		return errors.New("failed to create database entry")
	}

	LikePage(mod.Author, mod.ID)

	return nil
}

func UpdateModCachedLikes(modID string) error {
	count, err := GetLikes(modID)
	if err != nil {
		return err
	}
	_, err = Database.Exec("UPDATE mods SET likes=$1 WHERE id=$2 ", count, modID)
	if err != nil {
		return err
	}

	return nil
}

// Comment Management

func GetCommentCount(pageID string) (int, error) {
	row := Database.QueryRow("SELECT COUNT(*) FROM comments WHERE pageid=$1", pageID)
	var count int
	err := row.Scan(&count)
	return count, err
}

//

func HasRateLimit(ip string, typeLimit string, pageid string) bool {
	row := Database.QueryRow("SELECT COUNT(*) FROM ratelimits WHERE ip=$1, type=$2 pageid=$3", ip, typeLimit, pageid)
	var count int
	err := row.Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

func AddRateLimit(ip string, expirydate int64, typeLimit string, pageid string) error {
	_, err := Database.Exec("INSERT INTO ratelimits (ip, expirydate, pageid, type) VALUES ($1, $2, $3, $4)", ip, expirydate, typeLimit, pageid)
	return err
}
