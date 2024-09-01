package helper

import (
	"emlserver/structs"
	"errors"
	"os"
	"strconv"
)

func ParseMessage(message string) []structs.MessageElement {
	declareType := 0
	declareValue := 0

	typeBuffer := ""
	valueBuffer := ""
	plainTextBuffer := ""

	buffer := message

	var elements []structs.MessageElement

	for i := range buffer {
		char := buffer[i]

		if char == '(' && declareType == 0 {
			elements = append(elements, structs.MessageElement{
				Value: plainTextBuffer,
				Type:  "plain",
			})
			plainTextBuffer = ""
			declareType = 1
			continue
		}

		if char == ')' && declareType == 1 {
			declareType = 2
			continue
		}

		if char == '[' && declareType == 2 {
			declareValue = 1
			continue
		}

		if char == ']' && declareType == 2 && declareValue == 1 {
			declareValue = 2

			if _, err := strconv.Atoi(valueBuffer); err != nil {
				err := structs.MessageElement{
					Value: "Security Warning! Element Value was not numeric! Please contact moderators!",
					Type:  "plain",
				}
				elements = append(elements, err)
				return elements
			}

			element := structs.MessageElement{
				Value: valueBuffer,
				Type:  typeBuffer,
			}

			elements = append(elements, element)

			declareType = 0
			declareValue = 0
			valueBuffer = ""
			typeBuffer = ""

			continue
		}

		if declareValue == 1 {
			valueBuffer += string(char)
		} else if declareType == 1 {
			typeBuffer += string(char)
		} else if declareType == 0 && declareValue == 0 {
			plainTextBuffer += string(char)
		}
	}

	elements = append(elements, structs.MessageElement{
		Value: plainTextBuffer,
		Type:  "plain",
	})

	return elements
}

func CreateTemp() error {
	err := os.MkdirAll("tmp", 0777)

	if err != nil && errors.Is(err, os.ErrExist) {
		return err
	}

	return nil
}

func RemoveTemp() {
	err := os.RemoveAll("tmp")
	if err != nil {
		println("could not remove tmp")
	}
}
