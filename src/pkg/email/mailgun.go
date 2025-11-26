package email

import (
	"context"
	"fmt"
	"strings"

	"github.com/mailgun/mailgun-go/v4"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
)

/*
Attachment represents a file to be sent with the email.
Filename is what recipients will see; Data is the raw file bytes.
*/
type Attachment struct {
	Filename string
	Data     []byte
}

/*
SendMessageMailgunWithUnsubUrlAndAttachments sends an email via Mailgun with:
- HTML and/or plaintext bodies
- optional List-Unsubscribe headers
- zero or more attachments

All logging uses Blue for intent, Green for success, and includes recipient context.
*/
func SendMessageMailgunWithUnsubUrlAndAttachments(
	mailGunDomain, apiKey, senderAddress string, to []string, cc []string, bcc []string,
	subject, plainTextContent, htmlContent, unsubUrl string,
	attachments []Attachment,
) (err error, errMsg string) {
	toWhomString := strings.Join(to, ", ")
	if toWhomString == "" {
		toWhomString = "(no To recipients)"
	}

	tl.Log(tl.Info, palette.Blue, "Sending an email to '%s' using %s provider", toWhomString, "mailgun")

	mg := mailgun.NewMailgun(mailGunDomain, apiKey)

	message := mailgun.NewMessage(
		senderAddress,
		subject,
		plainTextContent,
		to...,
	)

	// Set HTML if present (can be the primary body).
	if htmlContent != "" {
		message.SetHTML(htmlContent)
	}

	// CC / BCC
	for _, addr := range cc {
		if addr != "" {
			message.AddCC(addr)
		}
	}
	for _, addr := range bcc {
		if addr != "" {
			message.AddBCC(addr)
		}
	}

	// Optional unsubscribe headers.
	if unsubUrl != "" {
		message.AddHeader("List-Unsubscribe", fmt.Sprintf("<%s>", unsubUrl))
		tl.Log(tl.Detailed, palette.Blue, "Set %s for an email to '%s'", "List-Unsubscribe header", unsubUrl)
		message.AddHeader("List-Unsubscribe-Post", "List-Unsubscribe=One-Click")
		tl.Log(tl.Detailed, palette.Blue, "Set %s for an email to '%s'", "List-Unsubscribe-Post header", "List-Unsubscribe=One-Click")
	}

	// Add attachments (if any).
	if len(attachments) > 0 {
		for _, att := range attachments {
			if len(att.Data) == 0 {
				continue
			}
			if att.Filename == "" {
				att.Filename = "attachment"
			}
			// Mailgun SDK convenience: send from memory buffer.
			message.AddBufferAttachment(att.Filename, att.Data)
		}
		tl.Log(tl.Verbose, palette.Cyan, "Attached %s file(s) for '%s'", len(attachments), toWhomString)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	response, id, sendErr := mg.Send(ctx, message)
	if sendErr != nil {
		return sendErr, fmt.Sprintf("Failed to send an email to '%s' using %s", toWhomString, "mailgun")
	}

	tl.Log(tl.Verbose, palette.Green, "Mailgun response ID: %s", id)
	tl.Log(tl.Verbose, palette.Green, "Mailgun response message: %s", response)

	if response == "Queued. Thank you." {
		tl.Log(tl.Info1, palette.Green, "Email sent successfully to '%s' using %s", toWhomString, "mailgun")
		return nil, ""
	}

	return fmt.Errorf("Unexpected response: %s", response), fmt.Sprintf("Email to '%s' using %s failed with response: %s", toWhomString, "mailgun", response)
}
