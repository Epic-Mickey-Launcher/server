package ticket

import (
	"database/sql"
	"emlserver/database"
	"emlserver/message"
	"emlserver/mod"
	"emlserver/security"
	"emlserver/structs"
	"fmt"
)

var DiscordReportTicket func(structs.Ticket)

const (
	AWAITING = 0
	APPROVED = 1
	DENIED   = -1

	AWAITING_STRING = "awaiting"
	APPROVED_STRING = "approved"
	DENIED_STRING   = "denied"

	REPORT              = "report"
	MOD_CHANGE_REPO_URL = "mod_change_repo_url"
)

func GetStringFromResult(result int) string {
	switch result {
	case AWAITING:
		return AWAITING_STRING
	case APPROVED:
		return APPROVED_STRING
	case DENIED:
		return DENIED_STRING
	}
	return "NaN"
}

func GetTicketFromTicketID(id string) (structs.Ticket, error) {
	row := database.Database.QueryRow("SELECT * FROM tickets WHERE ticketid=$1", id)
	return FormTicket(row)
}

func GetTicketFromTargetID(id string, action string) (structs.Ticket, error) {
	row := database.Database.QueryRow("SELECT * FROM tickets WHERE targetid=$1, action=$2", id, action)
	return FormTicket(row)
}

func DeleteTicket(id string) error {
	_, err := database.Database.Exec("DELETE FROM tickets WHERE ticketid=$1", id)
	return err
}

func AddTicket(title string, action string, targetID string, meta string, author string) error {
	id := security.GenerateID()
	_, err := database.Database.Exec("INSERT INTO tickets VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", id, action, targetID, 0, author, title, meta, "")
	if err != nil {
		return err
	}

	ticket, err := GetTicketFromTicketID(id)
	if err != nil {
		return err
	}

	DiscordReportTicket(ticket)
	return err
}

func CloseTicket(ticketID string, result int, resultMessage string) error {
	_, err := database.Database.Exec("UPDATE tickets SET result=$1, resultmsg=$2 WHERE ticketid=$3", result, resultMessage, ticketID)
	return err
}

func OnTicketReview(id string) error {
	ticket, err := GetTicketFromTicketID(id)
	if err != nil {
		return err
	}

	switch ticket.Action {
	case MOD_CHANGE_REPO_URL:
		message.SendMessage("0", ticket.Author, fmt.Sprintf("Your request to change (mod)[%s]'s Repository URL was %s.", ticket.TargetID, GetStringFromResult(ticket.Result)))
		if ticket.Result != APPROVED {
			break
		}
		go mod.HandleModRepository(ticket.Meta, mod.DOWNLOAD_AND_PACKAGE, ticket.TargetID, ticket.Author)
		break

	case REPORT:
		var response string

		if ticket.ResultMessage != "" {
			response = " Moderator Response: " + ticket.ResultMessage
		}

		err := message.SendMessage("0", ticket.Author, fmt.Sprintf("Your report on %s has been acknowledged by the moderation team and will be handled accordingly.", ticket.TargetID)+response)
		if err != nil {
			println(err.Error() + " (jamface)")
			return err
		}
		break
	}

	DeleteTicket(id)
	return nil
}

func FormTicket(row *sql.Row) (structs.Ticket, error) {
	var ticket structs.Ticket
	err := row.Scan(&ticket.ID, &ticket.Action, &ticket.TargetID, &ticket.Result, &ticket.Author, &ticket.Title, &ticket.Meta, &ticket.ResultMessage)
	return ticket, err
}
