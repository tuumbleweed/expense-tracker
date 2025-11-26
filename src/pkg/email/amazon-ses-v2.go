package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
)

const CharSet = "UTF-8"

// SendMessageAmazonSESV2 sends using "Simple" payload when there are no attachments.
// If attachments are provided, it automatically builds a multipart/mixed RAW email and sends it via RAW.
func SendMessageAmazonSESV2(
	awsRegion, senderAddress string, to []string, cc []string, bcc []string,
	subject, plainTextContent, htmlContent string, attachments []Attachment,
) (err error, errMsg string) {
	logWho := strings.Join(to, ", ")
	tl.Log(
		tl.Info, palette.Blue, "Sending an email to '%s' using %s provider",
		logWho, "Amazon SES (v2)",
	)

	// HTTP client with a global request timeout
	httpClient := awshttp.NewBuildableClient().WithTimeout(timeout)
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegion), config.WithHTTPClient(httpClient))
	if err != nil {
		return err, "Failed to load AWS config"
	}
	client := sesv2.NewFromConfig(cfg)

	// If there are attachments -> switch to RAW (multipart/mixed)
	if len(attachments) > 0 {
		rawBytes := buildRawMixedEmail(
			senderAddress, to, cc, bcc, /*bccHeader=*/ false, subject,
			plainTextContent, htmlContent, /*unsubURL*/ "",
			attachments,
		)

		input := &sesv2.SendEmailInput{
			FromEmailAddress: aws.String(senderAddress),
			Destination: &types.Destination{
				ToAddresses:  to,
				CcAddresses:  cc,
				BccAddresses: bcc, // still deliver Bcc; we do NOT add Bcc header
			},
			Content: &types.EmailContent{
				Raw: &types.RawMessage{Data: rawBytes},
			},
		}

		out, err := sesClientSendEmail(client, input, timeout)
		if err != nil {
			return err, fmt.Sprintf("Failed to send a raw email with attachments to '%s' using %s", logWho, "Amazon SES (v2)")
		}
		tl.Log(tl.Info1, palette.Green, "Email sent. MessageId='%s'", aws.ToString(out.MessageId))
		tl.Log(tl.Info1, palette.Green, "Email sent successfully to '%s' using %s", logWho, "Amazon SES (v2)")
		return nil, ""
	}

	// No attachments -> Simple path
	var body types.Body
	if htmlContent != "" {
		body.Html = &types.Content{Charset: aws.String(CharSet), Data: aws.String(htmlContent)}
	}
	if plainTextContent != "" {
		body.Text = &types.Content{Charset: aws.String(CharSet), Data: aws.String(plainTextContent)}
	}

	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(senderAddress),
		Destination: &types.Destination{
			ToAddresses:  to,
			CcAddresses:  cc,
			BccAddresses: bcc,
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Body: &body,
				Subject: &types.Content{
					Charset: aws.String(CharSet),
					Data:    aws.String(subject),
				},
			},
		},
	}

	out, err := sesClientSendEmail(client, input, timeout)
	if err != nil {
		return err, fmt.Sprintf("Failed to send an email to '%s' using %s", logWho, "Amazon SES (v2)")
	}
	tl.Log(tl.Info1, palette.Green, "Email sent. MessageId='%s'", aws.ToString(out.MessageId))
	tl.Log(tl.Info1, palette.Green, "Sent   an email to '%s' using %s provider", logWho, "Amazon SES (v2)")
	return nil, ""
}

// SendMessageAmazonSESRawV2 sends a raw MIME email (multipart/mixed if attachments present),
// allowing custom headers like List-Unsubscribe. To/Cc are rendered in headers.
// Bcc are delivered via Destination only (not revealed in headers).
func SendMessageAmazonSESRawV2(
	awsRegion, senderAddress string, to []string, cc []string, bcc []string,
	subject, plainTextContent, htmlContent, unsubURL string, attachments []Attachment,
) (err error, errMsg string) {
	logWho := strings.Join(to, ", ")
	tl.Log(
		tl.Info, palette.Blue, "Sending a raw email to '%s' using %s provider",
		logWho, "Amazon SES (v2)",
	)

	// HTTP client with a global request timeout
	httpClient := awshttp.NewBuildableClient().WithTimeout(timeout)
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(awsRegion), config.WithHTTPClient(httpClient))
	if err != nil {
		return err, "Failed to load AWS config"
	}
	client := sesv2.NewFromConfig(cfg)

	rawBytes := buildRawMixedEmail(
		senderAddress, to, cc, bcc, /*bccHeader=*/ false, subject,
		plainTextContent, htmlContent, unsubURL,
		attachments,
	)

	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(senderAddress),
		Destination: &types.Destination{
			ToAddresses:  to,
			CcAddresses:  cc,
			BccAddresses: bcc, // keep Bcc delivery without exposing header
		},
		Content: &types.EmailContent{
			Raw: &types.RawMessage{Data: rawBytes},
		},
	}

	out, err := sesClientSendEmail(client, input, timeout)
	if err != nil {
		return err, fmt.Sprintf("Failed to send a raw email to '%s' using %s", logWho, "Amazon SES (v2)")
	}
	tl.Log(tl.Info1, palette.Green, "Email sent. MessageId='%s'", aws.ToString(out.MessageId))
	tl.Log(tl.Info1, palette.Green, "Email sent successfully to '%s' using %s", logWho, "Amazon SES (v2)")
	return nil, ""
}

func sesClientSendEmail(client *sesv2.Client, input *sesv2.SendEmailInput, timeout time.Duration) (out *sesv2.SendEmailOutput, err error) {
	// Per-call timeout (optional, recommended)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return client.SendEmail(ctx, input)
}

// --- helpers ---------------------------------------------------------------

// buildRawMixedEmail creates a MIME message:
// multipart/mixed
//   ├─ multipart/alternative
//   │    ├─ text/plain (optional)
//   │    └─ text/html  (optional)
//   └─ attachment(s) (0+)
func buildRawMixedEmail(
	from string, to []string, cc []string, bcc []string,
	includeBccHeader bool, // usually false: do not expose Bcc
	subject, plainText, html, unsubURL string, attachments []Attachment,
) []byte {
	var (
		mixedBoundary = "mixedBoundaryEMV"
		altBoundary   = "altBoundaryEMV"
		sepMixed      = "--" + mixedBoundary
		sepAlt        = "--" + altBoundary
	)

	var buf bytes.Buffer

	// RFC 5322 headers
	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	if len(to) > 0 {
		buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	}
	if len(cc) > 0 {
		buf.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(cc, ", ")))
	}
	if includeBccHeader && len(to) == 0 && len(cc) == 0 { // rarely desirable; omitted by default
		buf.WriteString(fmt.Sprintf("Bcc: %s\r\n", strings.Join(bcc, ", ")))
	}
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", mixedBoundary))

	// Optional one-click unsubscribe (RFC 8058) — only possible in RAW
	if unsubURL != "" {
		buf.WriteString(fmt.Sprintf("List-Unsubscribe: <%s>\r\n", unsubURL))
		buf.WriteString("List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n")
	}
	buf.WriteString("\r\n") // end headers

	// multipart/alternative container
	buf.WriteString(sepMixed + "\r\n")
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", altBoundary))

	// text/plain
	if plainText != "" {
		buf.WriteString(sepAlt + "\r\n")
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
		buf.WriteString(plainText + "\r\n")
	}

	// text/html
	if html != "" {
		buf.WriteString(sepAlt + "\r\n")
		buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
		buf.WriteString(html + "\r\n")
	}

	// close multipart/alternative
	buf.WriteString(sepAlt + "--\r\n")

	// attachments
	for _, att := range attachments {
		if att.Filename == "" || len(att.Data) == 0 {
			continue
		}
		mimeType := http.DetectContentType(peek512(att.Data))
		if mimeType == "application/octet-stream" && strings.HasSuffix(strings.ToLower(att.Filename), ".txt") {
			mimeType = "text/plain"
		}

		buf.WriteString(sepMixed + "\r\n")
		buf.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", mimeType, qEncodeFilename(att.Filename)))
		buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", qEncodeFilename(att.Filename)))
		buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		buf.WriteString(chunkBase64(att.Data))
		buf.WriteString("\r\n")
		tl.Log(
			tl.Detailed, palette.CyanDim,
			"Attached file '%s' (%d bytes, type '%s')", att.Filename, len(att.Data), mimeType,
		)
	}

	// close multipart/mixed
	buf.WriteString(sepMixed + "--\r\n")

	return buf.Bytes()
}

// base64 with 76-char lines per RFC 2045 §6.8
func chunkBase64(b []byte) string {
	enc := base64.StdEncoding.EncodeToString(b)
	const line = 76
	if len(enc) <= line {
		return enc
	}
	var out strings.Builder
	for i := 0; i < len(enc); i += line {
		j := i + line
		if j > len(enc) {
			j = len(enc)
		}
		out.WriteString(enc[i:j])
		out.WriteString("\r\n")
	}
	return out.String()
}

// Encode filename safely if it has non-ASCII (simple Q-encoding of quotes not required for ASCII)
func qEncodeFilename(name string) string {
	// Keep it simple; most ASCII filenames are fine. If you expect UTF-8 names,
	// consider RFC 2231/5987; SES accepts standard quoted filenames widely.
	return strings.ReplaceAll(name, "\"", "'")
}
