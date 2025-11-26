//nolint:govet,staticcheck
package email

import (
	"bytes"
	"fmt"

	// those are deprecated, use v2 version
	// for now we will disable linting those lines and keep v1 until we switch permanently
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
)

// make sure AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY env vars are present
func SendMessageAmazonSES(awsRegion, senderAddress, recipientAddress, subject, plainTextContent, htmlContent string) (err error, errMsg string) {
	tl.Log(tl.Info, palette.Blue, "Sending an email to '%s' using %s provider", recipientAddress, "Amazon SES")
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	})
	if err != nil {
		return err, "Failed to create Amazon SES session:"
	}

	svc := ses.New(sess)

	body := ses.Body{}

	if htmlContent != "" {
		body.Html = &ses.Content{
			Charset: aws.String(CharSet),
			Data:    aws.String(htmlContent),
		}
	} else if plainTextContent != "" {
		body.Text = &ses.Content{
			Charset: aws.String(CharSet),
			Data:    aws.String(plainTextContent),
		}
	}

	input := &ses.SendEmailInput{
		Destination: &ses.Destination{
			ToAddresses: []*string{
				aws.String(recipientAddress),
			},
		},
		Message: &ses.Message{
			Body: &body,
			Subject: &ses.Content{
				Charset: aws.String(CharSet),
				Data:    aws.String(subject),
			},
		},
		Source: aws.String(senderAddress),
	}

	// Attempt to send the email
	_, err = svc.SendEmail(input)
	if err != nil {
		return err, fmt.Sprintf("Failed to send an email to '%s' using %s", recipientAddress, "Amazon SES")
	}

	tl.Log(tl.Info1, palette.Green, "Sent   an email to '%s' using %s provider", recipientAddress, "Amazon SES")

	return nil, ""
}


// raw version, allows to add custom headers such as List-Unsubscribe
func SendMessageAmazonSESRaw(awsRegion, senderAddress, recipientAddress, subject, plainTextContent, htmlContent, unsubUrl string) (err error, errMsg string) {
	tl.Log(tl.Info, palette.Blue, "Sending a raw email to '%s' using %s provider", recipientAddress, "Amazon SES")

	// Create AWS session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	})
	if err != nil {
		return err, "Failed to create Amazon SES session"
	}

	svc := ses.New(sess)

	// Build raw email body
	var emailBody bytes.Buffer

	// Standard headers
	emailBody.WriteString(fmt.Sprintf("From: %s\r\n", senderAddress))
	emailBody.WriteString(fmt.Sprintf("To: %s\r\n", recipientAddress))
	emailBody.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	emailBody.WriteString("MIME-Version: 1.0\r\n")
	emailBody.WriteString("Content-Type: multipart/alternative; boundary=\"boundary\"\r\n")

	// Add List-Unsubscribe header if unsubUrl is provided
	// also add List-Unsubscribe-Post: List-Unsubscribe=One-Click header
	// if unsubscribe url is provided
	if unsubUrl != "" {
		emailBody.WriteString(fmt.Sprintf("List-Unsubscribe: <%s>\r\n", unsubUrl))
		tl.Log(tl.Detailed, palette.Blue, "Set %s for an email to '%s'", "List-Unsubscribe header", unsubUrl)
		// List-Unsubscribe-Post: List-Unsubscribe=One-Click
		emailBody.WriteString("List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n")
		tl.Log(tl.Detailed, palette.Blue, "Set %s for an email to '%s'", "List-Unsubscribe-Post header", "List-Unsubscribe=One-Click")
	}
	emailBody.WriteString("\r\n")

	// MIME boundary
	boundary := "--boundary"

	// Plain-text body part
	emailBody.WriteString(fmt.Sprintf("%s\r\n", boundary))
	emailBody.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	emailBody.WriteString("\r\n")
	emailBody.WriteString(plainTextContent)
	emailBody.WriteString("\r\n")

	// HTML body part
	emailBody.WriteString(fmt.Sprintf("%s\r\n", boundary))
	emailBody.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	emailBody.WriteString("\r\n")
	emailBody.WriteString(htmlContent)
	emailBody.WriteString("\r\n")
	emailBody.WriteString(fmt.Sprintf("%s--\r\n", boundary))

	// Convert to SES RawMessage
	rawMessage := ses.RawMessage{
		Data: emailBody.Bytes(),
	}

	// Send the email
	input := &ses.SendRawEmailInput{
		RawMessage: &rawMessage,
		Source:     aws.String(senderAddress),
		Destinations: []*string{
			aws.String(recipientAddress),
		},
	}

	_, err = svc.SendRawEmail(input)
	if err != nil {
		return err, fmt.Sprintf("Failed to send a raw email to '%s' using %s", recipientAddress, "Amazon SES")
	}

	tl.Log(tl.Info1, palette.Green, "Email sent successfully to '%s' using %s", recipientAddress, "Amazon SES")

	return nil, ""
}
