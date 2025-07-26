package mail

import (
	"emlserver/config"
	"emlserver/database"
	"emlserver/structs"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/gomail.v2"
)

const (
	DONT_SEND_MESSAGES            = 0
	SEND_ALL_MESSAGES_EXCL_SYSTEM = 1
	SEND_ALL_MESSAGES             = 2

	DEFAULT_MESSAGES = SEND_ALL_MESSAGES_EXCL_SYSTEM

	MESSAGES_INDEX = 0
)

func FormatEmailOptions(messages int) (string, error) {
	if messages > 2 {
		err := errors.New("invalid messages index")
		return "", err
	}

	return fmt.Sprint(messages), nil
}

func UpdateEmailOptions(id string, messages int) error {
	row := database.Database.QueryRow("SELECT COUNT(*) FROM emailoptions WHERE id=$1", id)
	formattedOptions, err := FormatEmailOptions(messages)
	if err != nil {
		return err
	}

	var count int

	row.Scan(&count)

	if count == 0 {
		_, err = database.Database.Exec("INSERT INTO emailoptions VALUES ($1, $2)", id, formattedOptions)
	} else {
		_, err = database.Database.Exec("UPDATE emailoptions SET options=$1 WHERE id=$2", formattedOptions, id)
	}

	if err != nil {
		return err
	}

	return nil
}

func GetEmailOptions(id string) structs.EmailOptions {
	row := database.Database.QueryRow("SELECT * FROM emailoptions WHERE id=$1", id)

	var result structs.EmailOptions
	var optionsBuffer string

	err := row.Scan(&result.ID, &optionsBuffer)
	if err != nil {
		result.ID = id
		result.Messages = DEFAULT_MESSAGES
	}

	for i, e := range optionsBuffer {

		option, err := strconv.Atoi(string(e))
		switch i {
		case MESSAGES_INDEX:
			if err != nil {
				result.Messages = DEFAULT_MESSAGES
			} else {
				result.Messages = option
			}
			break
		}
	}

	return result
}

func VerifyEmail(to string, id string, key string) {
	user, err := database.GetUser(id)
	if err != nil {
		return
	}

	buffer, err := os.ReadFile("email/verifyEmail.html")
	if err != nil {
		return
	}

	strBuff := string(buffer)

	strBuff = strings.ReplaceAll(strBuff, "{USERNAME}", user.Username)
	strBuff = strings.ReplaceAll(strBuff, "{EMAIL}", to)
	strBuff = strings.ReplaceAll(strBuff, "{LINK}", fmt.Sprintf("%suser/email/auth?key=%s", config.LoadedConfig["URL"], key))
	SendMail(to, "Verify your E-Mail!", strBuff)
}

func ForgotPasswordEmail(to string, id string, token string) {
	user, err := database.GetUser(id)
	if err != nil {
		return
	}

	buffer, err := os.ReadFile("email/forgotpassword.html")
	if err != nil {
		return
	}

	strBuff := string(buffer)

	strBuff = strings.ReplaceAll(strBuff, "{USERNAME}", user.Username)
	strBuff = strings.ReplaceAll(strBuff, "{LINK}", fmt.Sprintf("%suser/otp/auth?token=%s", config.LoadedConfig["URL"], token))

	SendMail(to, "Forgot your Password?", strBuff)
}

func NewMessageEmail(fromUsername string, toUsername string, content string, email string) {
	buffer, err := os.ReadFile("email/newMessage.html")
	if err != nil {
		return
	}

	strBuff := string(buffer)
	strBuff = strings.ReplaceAll(strBuff, "{USERNAME}", toUsername)
	strBuff = strings.ReplaceAll(strBuff, "{SENDER}", fromUsername)
	strBuff = strings.ReplaceAll(strBuff, "{MESSAGE}", content)

	SendMail(email, "New message from "+fromUsername, strBuff)
}

func TestEmail(email string) error {
	buffer, err := os.ReadFile("email/test.html")
	if err != nil {
		return err
	}

	strBuff := string(buffer)
	strBuff = strings.ReplaceAll(strBuff, "{EMAIL}", strings.Split(email, "@")[0])
	return SendMail(email, "Test E-Mail", strBuff)
}

func SendMail(toEmail string, subject string, html string) error {
	port, err := strconv.Atoi(config.LoadedConfig["EMAIL_PORT"])
	if err != nil {
		panic(err)
	}
	dialer := gomail.NewDialer(config.LoadedConfig["EMAIL_HOST"], port, config.LoadedConfig["EMAIL_ADDRESS"], config.LoadedConfig["EMAIL_PASSWORD"])
	message := gomail.NewMessage()
	message.SetHeader("From", config.LoadedConfig["EMAIL_ADDRESS"])
	message.SetHeader("To", toEmail)
	message.SetHeader("Subject", subject)
	message.SetBody("text/html", html)

	err = dialer.DialAndSend(message)
	println("Sent E-Mail (", subject, ") to ", toEmail)

	if err != nil {
		fmt.Printf("E-Mail Send Failure: [Error: %s] [Subject: %s]", err.Error(), subject)
	}

	return err
}
