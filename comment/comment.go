package comment

import (
	"emlserver/database"
	"emlserver/message"
	"emlserver/security"
	"emlserver/structs"
	"slices"

	"github.com/TwiN/go-away"
)

func SendComment(id string, pageID string, content string) error {
	message.SendMessage("0", id, "Please mind your language when commenting.")
	filteredContent := goaway.Censor(content)
	commentID := security.GenerateID()
	_, err := database.Database.Exec("INSERT INTO comments (pageid, content, id, author) VALUES ($1, $2, $3, $4)", pageID, filteredContent, commentID, id)
	return err
}

func DeleteComment(ID string) error {
	_, err := database.Database.Exec("DELETE FROM comments WHERE id=$1", ID)
	return err
}

func QueryComments(pageID string) ([]structs.Comment, error) {
	rows, err := database.Database.Query("SELECT pageid, content, id, author FROM comments WHERE pageid=$1", pageID)
	if err != nil {
		return nil, err
	}

	var comments []structs.Comment

	for rows.Next() {
		var comment structs.Comment

		err = rows.Scan(&comment.Page, &comment.Content, &comment.ID, &comment.Author)
		if err != nil {
			return nil, err
		}

		comments = append(comments, comment)
	}

	slices.Reverse(comments)

	return comments, nil
}

func GetComment(id string) (structs.Comment, error) {
	row := database.Database.QueryRow("SELECT pageid, content, id, author FROM comments WHERE id=$1", id)
	var comment structs.Comment
	err := row.Scan(&comment.Page, &comment.Content, &comment.ID, &comment.Author)
	return comment, err
}
