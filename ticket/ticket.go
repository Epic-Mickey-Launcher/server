package ticket

import (
	"database/sql"
	"emlserver/database"
	"emlserver/message"
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

	REPORT    = "report"
	MODREVIEW = "modreview"
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

	var result string
	switch ticket.Result {
	case APPROVED:
		result = APPROVED_STRING
	case DENIED:
		result = DENIED_STRING
	}
	var response string

	if ticket.ResultMessage != "" {
		response = " Moderator Response: " + ticket.ResultMessage
	}

	switch ticket.Action {
	case REPORT:
		err := message.SendMessage("0", ticket.Author, fmt.Sprintf("Your report on %s has been acknowledged by the moderation team and will be handled accordingly.", ticket.TargetID)+response)
		if err != nil {
			println(err.Error() + " (jamface)")
			return err
		}
	case MODREVIEW:

		if ticket.Result == DENIED {
			err := database.DeleteMod(ticket.TargetID)
			if err != nil {

				println(err.Error() + " (tiestow)")
				return err
			}
		} else if ticket.Result == APPROVED {
			database.VerifyMod(ticket.TargetID)
		}

		err = message.SendMessage("0", ticket.Author, fmt.Sprintf("Your mod <%s> has been reviewed. It has been %s. ", ticket.TargetID, result)+response)
		if err != nil {
			println(err.Error() + " (trigo)")
			return err
		}
	}

	DeleteTicket(id)
	return nil
}

func FormTicket(row *sql.Row) (structs.Ticket, error) {
	var ticket structs.Ticket
	err := row.Scan(&ticket.ID, &ticket.Action, &ticket.TargetID, &ticket.Result, &ticket.Author, &ticket.Title, &ticket.Meta, &ticket.ResultMessage)
	return ticket, err
}
