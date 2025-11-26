package email

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
)

// Supports To/Cc/Bcc, optional List-Unsubscribe, and attachments.
func SendMessageSendgrid(
	apiKey, senderAddress string, to []string, cc []string, bcc []string,
	subject, plainTextContent, htmlContent, unsubUrl string, attachments []Attachment,
) (err error, errMsg string) {
	logWho := strings.Join(to, ", ")
	if logWho == "" {
		logWho = "(no To recipients)"
	}
	tl.Log(tl.Info, palette.Blue, "Sending an email to '%s' using %s provider", logWho, "sendgrid")

	// Build V3 Mail (supports multiple recipients)
	msg := mail.NewV3Mail()
	msg.SetFrom(mail.NewEmail("", senderAddress))
	msg.Subject = subject

	// Add content (at least one)
	if plainTextContent != "" {
		msg.AddContent(mail.NewContent("text/plain", plainTextContent))
	}
	if htmlContent != "" {
		msg.AddContent(mail.NewContent("text/html", htmlContent))
	}

	// Personalization with To/Cc/Bcc
	p := mail.NewPersonalization()
	for _, addr := range to {
		p.AddTos(mail.NewEmail("", addr))
	}
	for _, addr := range cc {
		p.AddCCs(mail.NewEmail("", addr))
	}
	for _, addr := range bcc {
		p.AddBCCs(mail.NewEmail("", addr))
	}

	// Optional List-Unsubscribe headers (per RFC 8058)
	if unsubUrl != "" {
		p.SetHeader("List-Unsubscribe", fmt.Sprintf("<%s>", unsubUrl))
		tl.Log(tl.Detailed, palette.Blue, "Set %s for an email to '%s'", "List-Unsubscribe header", unsubUrl)
		p.SetHeader("List-Unsubscribe-Post", "List-Unsubscribe=One-Click")
		tl.Log(tl.Detailed, palette.Blue, "Set %s for an email to '%s'", "List-Unsubscribe-Post header", "List-Unsubscribe=One-Click")
	}

	msg.AddPersonalizations(p)

	// Attachments
	for _, att := range attachments {
		if att.Filename == "" || len(att.Data) == 0 {
			continue // skip empties
		}

		sgAtt := mail.NewAttachment()
		sgAtt.SetFilename(att.Filename)
		sgAtt.SetDisposition("attachment")

		// Best-effort MIME type detection
		mime := http.DetectContentType(peek512(att.Data))
		sgAtt.SetType(mime)

		// Base64 content as required by SendGrid
		sgAtt.SetContent(base64.StdEncoding.EncodeToString(att.Data))

		msg.AddAttachment(sgAtt)

		tl.Log(
			tl.Detailed, palette.CyanDim,
			"Attached file '%s' (%d bytes, type '%s')", att.Filename, len(att.Data), mime,
		)
	}
	
	resp, err := sendWithTimeout(apiKey, msg, timeout)
	if err != nil {
		return err, fmt.Sprintf("Failed to send an email to '%s' using %s", logWho, "sendgrid")
	}

	tl.Log(tl.Verbose, palette.CyanDim, "SendGrid response status code: %d", resp.StatusCode)
	tl.Log(tl.Verbose, palette.CyanDim, "SendGrid response body: %s", resp.Body)
	tl.Log(tl.Verbose, palette.CyanDim, "SendGrid response headers: %v", resp.Headers)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		tl.Log(tl.Info1, palette.Green, "Email sent successfully to '%s' using %s", logWho, "sendgrid")
		return nil, ""
	}

	return fmt.Errorf("unexpected response code: %d", resp.StatusCode),
		fmt.Sprintf("Email to '%s' using %s failed with status code: %d", logWho, "sendgrid", resp.StatusCode)
}

// peek512 returns up to the first 512 bytes (used for MIME sniffing).
func peek512(b []byte) []byte {
	if len(b) > 512 {
		return b[:512]
	}
	return b
}


func newSendGridClient(apiKey string, timeout time.Duration) *sendgrid.Client {
	// Global client used by sendgrid-go → rest.DefaultClient → http.Client
	rest.DefaultClient.HTTPClient.Timeout = timeout
	return sendgrid.NewSendClient(apiKey)
}

func sendWithTimeout(apiKey string, msg *mail.SGMailV3, timeout time.Duration) (resp *rest.Response, err error) {
	client := newSendGridClient(apiKey, timeout)

	// (Optional) also enforce a per-call timeout:
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return client.SendWithContext(ctx, msg)
}
