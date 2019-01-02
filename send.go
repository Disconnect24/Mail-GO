package main

import (
	"bufio"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/smtp"
	"regexp"
	"strings"

	"github.com/Disconnect24/Mail-GO/utilities"
	"github.com/google/uuid"
)

var mailFormName = regexp.MustCompile(`m\d+`)
var mailFrom = regexp.MustCompile(`^MAIL FROM:\s(w[0-9]*)@(?:.*)$`)
var rcptFrom = regexp.MustCompile(`^RCPT TO:\s(.*)@(.*)$`)

// Send takes POSTed mail by the Wii and stores it in the database for future usage.
func Send(c *gin.Context) {
	c.Header("Content-Type", "text/plain;charset=utf-8")
	// Go ahead and prepare the insert statement, for later usage.
	stmt, err := db.Prepare("INSERT INTO `mails` (`sender_wiiID`,`mail`, `recipient_id`, `mail_id`, `message_id`) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		// Welp, that went downhill fast.
		ErrorResponse(c, 450, "Database error.")
		utilities.LogError(ravenClient, "Prepared send statement error", err)
		return
	}

	// Create maps for storage of mail.
	mailPart := make(map[string]string)

	// Parse form in preparation for finding mail.
	form, err := c.MultipartForm()
	if err != nil {
		ErrorResponse(c, 350, "Failed to parse mail.")
		utilities.LogError(ravenClient, "Failed to parse mail", err)
		return
	}

	// Now check if it can be verified
	isVerified, err := AuthForSend(c.PostForm("mlid"))
	if err != nil {
		ErrorResponse(c, 551, "Something weird happened.")
		utilities.LogError(ravenClient, "Error changing from authentication database.", err)
		return
	} else if !isVerified {
		ErrorResponse(c, 240, "An authentication error occurred.")
		return
	}

	for name, contents := range form.Value {
		if mailFormName.MatchString(name) {
			mailPart[name] = contents[0]
		}
	}

	eventualOutput := SuccessfulResponse
	eventualOutput += fmt.Sprint("mlnum=", len(mailPart), "\n")

	// Handle all the mail! \o/
	for mailNumber, contents := range mailPart {
		var wiiRecipientIDs []string
		var pcRecipientIDs []string
		// senderID must be a string.
		// The database contains `w<16 digit ID>` due to previous PHP scripts.
		// POTENTIAL TODO: remove w from database?
		var senderID string
		var mailContents string

		// For every new line, handle as needed.
		scanner := bufio.NewScanner(strings.NewReader(contents))
		for scanner.Scan() {
			line := scanner.Text()

			if line == "DATA" {
				// This line just tells the server beyond here to stop processing
				// We shouldn't send that to the client, so we're done.
				break
			}

			potentialMailFromWrapper := mailFrom.FindStringSubmatch(line)
			if potentialMailFromWrapper != nil {
				potentialMailFrom := potentialMailFromWrapper[1]
				// "Special" number from Nintendo, used to send to allusers@wii.com.
				// While not necessarily hardcoded anywhere, no need to confuse.
				if potentialMailFrom == "w9999999900000000" {
					eventualOutput += MailErrorResponse(351, "w9999999900000000 tried to send mail.", mailNumber)
					return
				}
				senderID = potentialMailFrom
				continue
			}

			// -1 signifies all matches
			potentialRecipientWrapper := rcptFrom.FindAllStringSubmatch(line, -1)
			if potentialRecipientWrapper != nil {
				// We only need to work with the first match, which should be all we need.
				potentialRecipient := potentialRecipientWrapper[0]

				// layout:
				// potentialRecipient[1] = w<16 digit ID>
				// potentialRecipient[2] = domain being sent to
				if potentialRecipient[2] == "wii.com" {
					// We're not gonna allow you to send to a defunct domain. ;P
				} else if potentialRecipient[2] == global.SendGridDomain {
					// Wii <-> Wii mail. We can handle this.
					wiiRecipientIDs = append(wiiRecipientIDs, potentialRecipient[1])
				} else {
					// PC <-> Wii mail. An actual mail server will handle this.
					email := fmt.Sprintf("%s@%s", potentialRecipient[1], potentialRecipient[2])
					pcRecipientIDs = append(pcRecipientIDs, email)
				}
			}

			// This line doesn't need to be processed and can be added.
			mailContents += line
		}

		if err := scanner.Err(); err != nil {
			eventualOutput += MailErrorResponse(551, "Issue iterating over strings.", mailNumber)
			utilities.LogError(ravenClient, "Error reading from scanner", err)
			return
		}

		// Replace all @wii.com references in the friend request email with our own domain.
		// Format: w9004342343324713@wii.com <mailto:w9004342343324713@wii.com>
		mailContents = strings.Replace(mailContents,
			fmt.Sprintf("%s@wii.com <mailto:%s@wii.com>", senderID, senderID),
			fmt.Sprintf("%s@%s <mailto:%s@%s>", senderID, global.SendGridDomain, senderID, global.SendGridDomain),
			-1)

		// We're done figuring out the mail, now it's time to act as needed.
		// For Wii recipients, we can just insert into the database.
		for _, wiiRecipient := range wiiRecipientIDs {
			// Splice wiiRecipient to drop w from 16 digit ID.
			_, err := stmt.Exec(senderID, mailContents, wiiRecipient[1:], uuid.New().String(), uuid.New().String())
			if err != nil {
				eventualOutput += MailErrorResponse(450, "Database error.", mailNumber)
				utilities.LogError(ravenClient, "Error inserting mail", err)
				return
			}
		}

		for _, pcRecipient := range pcRecipientIDs {
			err := handlePCmail(senderID, pcRecipient, mailContents)
			if err != nil {
				utilities.LogError(ravenClient, "Error sending mail via SMTP", err)
				eventualOutput += MailErrorResponse(551, "Issue sending mail via SMTP.", mailNumber)
				return
			}
		}
		eventualOutput += MailErrorResponse(100, "Success.", mailNumber)
	}

	// We're completely done now.
	c.String(http.StatusOK, eventualOutput)
}

func handlePCmail(senderID string, pcRecipient string, mailContents string) error {
	// Connect to the remote SMTP server.
	host := "smtp.sendgrid.net"
	auth := smtp.PlainAuth(
		"",
		"apikey",
		global.SendGridKey,
		host,
	)
	// The only reason we can get away with the following is
	// because the Wii POSTs valid SMTP syntax.
	return smtp.SendMail(
		fmt.Sprint(host, ":587"),
		auth,
		fmt.Sprintf("%s@%s", senderID, global.SendGridDomain),
		[]string{pcRecipient},
		[]byte(mailContents),
	)

}
