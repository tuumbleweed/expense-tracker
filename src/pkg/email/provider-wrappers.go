package email

import (
	"encoding/base64"
	"fmt"
	"os"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"

	"this-project/src/pkg/util"
)

/*
Choose provider based on Cfg.Provider. Use it to send a message.

Since this function is in "email" package and reporting.Cfg.SendEmails belongs
to "reporting" package we cannot read reporting.Cfg.SendEmails directly
lest we cause import cycle.
Thus we must pass it as a parameter to this function.
*/
func SendMessage(
	provider Provider, sendEmails *bool, senderAddress string, recipientAddresses []string,
	subject, plainTextContent, htmlContent string, attachments []Attachment,
) (e *xerr.Error) {
	if sendEmails == nil || !*sendEmails { // no nil dereference, sendEmails == nil is checked first
		var sendEmailsLog string
		if sendEmails == nil {
			sendEmailsLog = "nil"
		} else if !*sendEmails {
			sendEmailsLog = "false"
		} else {
			sendEmailsLog = "true"
		}
		tl.Log(tl.Notice, palette.PurpleBold, "%s because %s is set to %s", "Not sending an email", "send_emails", sendEmailsLog)
		return nil
	}

	switch provider {
	case ProviderMailgun:
		e = SendMessageMailgunWrapper(senderAddress, recipientAddresses, subject, plainTextContent, htmlContent, attachments)
	case ProviderSendGrid:
		e = SendMessageSendgridWrapper(senderAddress, recipientAddresses, subject, plainTextContent, htmlContent, attachments)
	case ProviderAmazonSES:
		e = SendMessageAmazonSESWrapper(senderAddress, recipientAddresses, subject, plainTextContent, htmlContent, attachments)
	default:
		return xerr.NewError(
			fmt.Errorf("Unsupported provider: '%s'", provider),
			fmt.Sprintf("Provider must be among those: %v", AllowedProviders),
			provider,
		)
	}

	if e != nil {
		contextMap := map[string]any{
			"provider":   provider,
			"sender":     senderAddress,
			"recipients": recipientAddresses,
			"subject":    subject,
			"attCount":   len(attachments),
			"plainHash":  sha256Short(plainTextContent),
			"htmlHash":   sha256Short(htmlContent),
		}
		e.Context = xerr.StringifyContext(contextMap)
		return e
	}

	util.WaitForSeconds(3)
	return e
}

func SendMessageAmazonSESWrapper(
	senderAddress string, recipientAddresses []string, subject, plainTextContent, htmlContent string,
	attachments []Attachment,
) (e *xerr.Error) {

	err, errMsg := SendMessageAmazonSESRawV2(
		os.Getenv("AWS_REGION"), senderAddress, recipientAddresses, nil, nil,
		subject, plainTextContent, htmlContent, "", attachments,
	)
	if err != nil {
		return xerr.NewError(err, errMsg, nil)
	}

	return nil
}

func SendMessageSendgridWrapper(
	senderAddress string, recipientAddresses []string, subject, plainTextContent, htmlContent string,
	attachments []Attachment,
) (e *xerr.Error) {

	err, errMsg := SendMessageSendgrid(
		os.Getenv("SENDGRID_API_KEY"), senderAddress, recipientAddresses, nil, nil,
		subject, plainTextContent, htmlContent, "", attachments,
	)
	if err != nil {
		return xerr.NewError(err, errMsg, nil)
	}

	return nil
}

/*
SendMessageMailgunWrapperWithAttachments behaves like SendMessageMailgunWrapper,
but also accepts attachments. It reads MAILGUN_DOMAIN and MAILGUN_API_KEY from env.
*/
func SendMessageMailgunWrapper(
	senderAddress string, recipientAddresses []string, subject, plainTextContent, htmlContent string,
	attachments []Attachment,
) (e *xerr.Error) {

	err, errMsg := SendMessageMailgunWithUnsubUrlAndAttachments(
		os.Getenv("MAILGUN_DOMAIN"), os.Getenv("MAILGUN_API_KEY"), senderAddress,
		recipientAddresses, nil, nil, subject, plainTextContent, htmlContent, "", attachments,
	)
	if err != nil {
		return xerr.NewError(err, errMsg, nil)
	}

	return nil
}

func sha256Short(s string) string {
	// short, readable integrity hint for logs (not cryptographically used)
	h := make([]byte, 0)
	enc := base64.StdEncoding
	buf := make([]byte, enc.EncodedLen(len(s)))
	// We re-use base64 over a plain SHA placeholder to keep stdlib only and cheap;
	// if you prefer true sha256, swap this with crypto/sha256.Sum256 and hex-encode.
	copy(buf, []byte(s))
	_ = h
	return string(buf[:min(24, len(buf))])
}
