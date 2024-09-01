package message

import (
	"emlserver/database"
	"emlserver/mail"
	"emlserver/security"
	"emlserver/structs"
)

func SendMessage(from string, to string, content string) error {
	user, err := database.GetUser(to)
	if err != nil {
		return err
	}

	id := security.GenerateID()
	_, err = database.Database.Exec("INSERT INTO messages VALUES ($1, $2, $3, $4)", id, security.Encrypt(content), from, to)
	if err != nil {
		println(err.Error())
	}

	if user.Email != "" {
		mailOptions := mail.GetEmailOptions(user.ID)

		if mailOptions.Messages == mail.SEND_ALL_MESSAGES || mailOptions.Messages == mail.SEND_ALL_MESSAGES_EXCL_SYSTEM && from != "0" {

			var fromUsername string

			if from == "0" {
				fromUsername = "System"
			} else {
				userObj, err := database.GetUser(from)
				if err != nil {
					return err
				}
				fromUsername = userObj.Username
			}

			decryptedMail := security.Decrypt(user.Email)

			mail.NewMessageEmail(fromUsername, user.Username, content, decryptedMail)
		}
	}

	return nil
}

func GetMessage(id string) (structs.Message, error) {
	var msg structs.Message
	row := database.Database.QueryRow("SELECT id, content, fromid, toid FROM messages WHERE id=$1", id)
	err := row.Scan(&msg.ID, &msg.Content, &msg.From, &msg.To)
	msg.Content = security.Decrypt(msg.Content)
	return msg, err
}

func DeleteMessage(id string) error {
	_, err := database.Database.Exec("DELETE FROM messages WHERE id=$1", id)
	return err
}

func GetMessagesForUser(userID string) ([]structs.Message, error) {
	rows, err := database.Database.Query("SELECT id FROM messages WHERE toid=$1", userID)
	var msgs []structs.Message
	if err != nil {
		println(err.Error())
		return msgs, err
	}

	for rows.Next() {
		var msg structs.Message
		err = rows.Scan(&msg.ID)
		if err != nil {
			return msgs, err
		}
		msg, err = GetMessage(msg.ID)
		if err != nil {
			return msgs, err
		}
		msgs = append(msgs, msg)
	}

	return msgs, nil
}
