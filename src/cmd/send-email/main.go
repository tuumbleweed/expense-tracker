// in case you need to create an entrypoint with multiple subprograms
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
	"github.com/tuumbleweed/xerr"

	"expense-tracker/src/pkg/config"
	"expense-tracker/src/pkg/email"
	"expense-tracker/src/pkg/util"
)

/*
Pick prvider and use it to send a test email to admin/specified address.
Specify test email file path (generate it with substitute-variables subprogram)
*/
func testProvider(subprogram string, flags []string) {
	config.CheckIfEnvVarsPresent(
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION", // amazon ses
		"MAILGUN_DOMAIN", "MAILGUN_API_KEY", // mailgun
		"SENDGRID_API_KEY", // sendgrid
	)

	// common flags
	subprogramCmd := flag.NewFlagSet(subprogram, flag.ExitOnError)
	configPath := subprogramCmd.String("config", "./cfg/config.json", "Log level. Default is LOG_LEVEL env var value")

	// custom flags
	provider := subprogramCmd.String("provider", "mailgun", "Provider to use when sending emails")
	senderAddress := subprogramCmd.String("sender", "", "Sender's address")
	recipientAddress := subprogramCmd.String("recipient", "", "Recipient's address")
	subject := subprogramCmd.String("subject", "Test subject", "Subject of an email")
	emailHtmlFilePath := subprogramCmd.String("html", "./tmp/email.html", "Html of an email, with variables substituted")
	emailTextFilePath := subprogramCmd.String("text", "./tmp/email.txt", "Html of an email, with variables substituted")

	// parse and init config
	xerr.QuitIfError(subprogramCmd.Parse(flags), "Unable to subprogramCmd.Parse")
	config.InitializeConfig(*configPath)

	util.RequiredFlag(senderAddress, "sender")
	util.RequiredFlag(recipientAddress, "recipient")
	util.RequiredFlag(provider, "provider")
	util.EnsureFlags()

	recipientAddresses := strings.Split(*recipientAddress, ",")

	// read html file
	htmlFileContentBytes, err := os.ReadFile(*emailHtmlFilePath)
	xerr.QuitIfError(err, fmt.Sprintf("Unable to read file '%s'", *emailHtmlFilePath))
	tl.Log(tl.Verbose, palette.BlueDim, "Full Email:\n```\n%s\n```", htmlFileContentBytes)
	// read text file
	textFileContentBytes, err := os.ReadFile(*emailTextFilePath)
	xerr.QuitIfError(err, fmt.Sprintf("Unable to read file '%s'", *emailTextFilePath))
	tl.Log(tl.Verbose, palette.BlueDim, "Full Email:\n```\n%s\n```", textFileContentBytes)

	// send email here
	sendEmails := true
	e := email.SendMessage(email.Provider(*provider), &sendEmails, *senderAddress, recipientAddresses, *subject, string(textFileContentBytes), string(htmlFileContentBytes), nil)
	e.QuitIf("error")
}

func main() {
	// Check if there are enough arguments
	if len(os.Args) < 2 {
		tl.Log(tl.Error, palette.Red, "Usage: %s", "go run src/cmd/unsubscriber/main.go subprogram_name(for exampe unsubscriber)")
		os.Exit(1)
	}
	subprogram := os.Args[1]
	flags := os.Args[2:]

	// Switch subprogram based on the first argument
	switch subprogram {
	case "test-provider":
		testProvider(subprogram, flags)
	default:
		tl.Log(tl.Error, palette.Red, "Unknown subprogram: %s", subprogram)
		os.Exit(1)
	}
}
