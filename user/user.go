package user

import (
	"database/sql"
	"emlserver/database"
	"emlserver/security"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	goaway "github.com/TwiN/go-away"
)

func GetProfilePicturePath(id string) (string, bool) {
	path := fmt.Sprint("static/pfp/", id, ".webp")
	_, err := os.Open(path)
	if err != nil {
		path = fmt.Sprint("static/pfp/default.webp")
	}
	return path, err != nil
}

func ValidateUsername(username string) error {
	if len(strings.TrimSpace(username)) < 3 {
		return errors.New("username is too short")
	}

	if strings.Contains(username, " ") {
		return errors.New("username cannot contain spaces")
	}

	isNormal := isASCII(username)
	if !isNormal {
		return errors.New("username can only have ascii characters")
	}

	if goaway.IsProfane(username) {
		return errors.New("username cannot have bad words in it")
	}

	existingUser := ""
	err := database.Database.QueryRow("SELECT username FROM users WHERE username = $1", username).Scan(&existingUser)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return errors.New("db error")
	}

	if existingUser != "" {
		return errors.New("a user has already claimed this username")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(strings.TrimSpace(password)) < 8 {
		return errors.New("password is too short (<8)")
	}
	if len(strings.TrimSpace(password)) > 72 {
		return errors.New("password is too long (>72)")
	}
	isNormal := isASCII(password)
	if !isNormal {
		return errors.New("password can only have ascii characters")
	}
	return nil
}

func GetUserWithToken(token string) (string, error) {
	var id string
	err := database.Database.QueryRow("SELECT id FROM users WHERE token = $1", token).Scan(&id)
	if err != nil {
		return "", err
	}

	return id, nil
}

func GetOneTimePassword(userid string) (error, string) {
	row := database.Database.QueryRow("SELECT otp FROM otp WHERE userid=$1", userid)
	var otp string
	err := row.Scan(&otp)
	database.Database.Exec("DELETE FROM otp WHERE userid=$1", userid)
	return err, otp
}

func LoginUser(username string, password string) (string, error) {
	var existingUserPass string
	var userid string
	err := database.Database.QueryRow("SELECT password, id FROM users WHERE username = $1 OR emailhash=$2", username, security.Hash(username)).Scan(&existingUserPass, &userid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("user does not exist")
		}
		return "", errors.New("database error")
	}

	err, otp := GetOneTimePassword(userid)

	var otpValid bool = false

	if err == nil {
		otpValid = security.CompareHashToString(password, otp)
	}

	if security.PasswordsMatch(existingUserPass, password) || otpValid {
		token, err := database.GenerateUserToken(userid)
		if err != nil {
			return "", err
		}
		return token, nil
	}
	return "", errors.New("invalid password")
}

func CreateUser(username string, password string) (string, error) {
	println("Attempting to register user: ", username)
	err := ValidateUsername(username)
	if err != nil {
		return "", err
	}
	err = ValidatePassword(password)
	if err != nil {
		return "", err
	}
	user, err := database.CreateUser()
	if err != nil {
		println("Failed to create user " + err.Error())
		return "", err
	}

	database.SetUsername(user, strings.TrimSpace(username))
	err = database.SetPassword(user, strings.TrimSpace(password))
	if err != nil {
		return "", err
	}
	database.SetBio(user, "A new member!")

	err = database.InitEmail(user)
	if err != nil {
		return "", err
	}

	return database.GenerateUserToken(user)
}

func DeleteUser(userid string) error {
	path, def := GetProfilePicturePath(userid)

	if !def {
		err := os.Remove(path)
		if err != nil {
			println("Could not remove profile picture?")
		}
	}

	return database.DeleteUser(userid)
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}
