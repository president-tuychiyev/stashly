// Package mail renders the messages this service sends. The bodies are built
// from Go templates instead of view files so that a missing resources
// directory can never break a login flow at runtime: the templates are parsed
// once at start up and a parse error would be a panic in development, not a
// 500 in production.
package mail

import (
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
)

// Subject is the subject line every one time code message carries. The
// contract pins it, the web app and the tests match on it.
const Subject = "Your Stashly verification code"

// Data is what every template is rendered with.
type Data struct {
	// Name is the recipient's display name, may be empty.
	Name string
	// Code is the six digit one time code.
	Code string
	// Minutes is how long the code stays valid.
	Minutes int
	// NewEmail is the address an email change would move the account to.
	NewEmail string
	// AppName is the service name shown in the body.
	AppName string
}

// Message is a rendered mail, ready to hand to a mailer.
type Message struct {
	Subject string
	Text    string
	HTML    string
}

const greeting = `{{if .Name}}Hello {{.Name}},{{else}}Hello,{{end}}`

const verifyText = greeting + `

Welcome to {{.AppName}}. Use the code below to activate your account and set your password.

    {{.Code}}

The code is valid for {{.Minutes}} minutes and can be used once.
If you did not expect this email you can ignore it.

-- {{.AppName}}
`

const passwordResetText = greeting + `

We received a request to reset the password of your {{.AppName}} account.
Use the code below to choose a new password.

    {{.Code}}

The code is valid for {{.Minutes}} minutes and can be used once.
If you did not request this, ignore this email; your password stays unchanged.

-- {{.AppName}}
`

const emailChangeText = greeting + `

Use the code below to confirm {{.NewEmail}} as the new address of your {{.AppName}} account.

    {{.Code}}

The code is valid for {{.Minutes}} minutes and can be used once.
If you did not ask for this change, ignore this email; the address stays unchanged.

-- {{.AppName}}
`

const htmlLayout = `<!doctype html>
<html lang="en">
<body style="margin:0;padding:24px;background:#f5f6f8;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;color:#1f2933;">
  <table role="presentation" style="max-width:520px;margin:0 auto;background:#ffffff;border-radius:8px;padding:32px;">
    <tr><td>
      <p style="margin:0 0 16px;">{{if .Name}}Hello {{.Name}},{{else}}Hello,{{end}}</p>
      <p style="margin:0 0 24px;">{{.Intro}}</p>
      <p style="margin:0 0 24px;font-size:32px;letter-spacing:8px;font-weight:700;text-align:center;">{{.Code}}</p>
      <p style="margin:0 0 8px;color:#52606d;font-size:14px;">The code is valid for {{.Minutes}} minutes and can be used once.</p>
      <p style="margin:0;color:#52606d;font-size:14px;">{{.Footer}}</p>
    </td></tr>
  </table>
</body>
</html>
`

type htmlData struct {
	Name    string
	Code    string
	Minutes int
	Intro   string
	Footer  string
}

var (
	verifyTextTemplate        = texttemplate.Must(texttemplate.New("verify.txt").Parse(verifyText))
	passwordResetTextTemplate = texttemplate.Must(texttemplate.New("password_reset.txt").Parse(passwordResetText))
	emailChangeTextTemplate   = texttemplate.Must(texttemplate.New("email_change.txt").Parse(emailChangeText))
	htmlTemplate              = htmltemplate.Must(htmltemplate.New("layout.html").Parse(htmlLayout))
)

// Verification renders the message that activates an invited user.
func Verification(data Data) (Message, error) {
	return render(verifyTextTemplate, data, htmlData{
		Intro:  "Welcome to " + data.AppName + ". Use the code below to activate your account and set your password.",
		Footer: "If you did not expect this email you can ignore it.",
	})
}

// PasswordReset renders the message that lets a user choose a new password.
func PasswordReset(data Data) (Message, error) {
	return render(passwordResetTextTemplate, data, htmlData{
		Intro:  "We received a request to reset the password of your " + data.AppName + " account. Use the code below to choose a new password.",
		Footer: "If you did not request this, ignore this email; your password stays unchanged.",
	})
}

// EmailChange renders the message that confirms a new address.
func EmailChange(data Data) (Message, error) {
	return render(emailChangeTextTemplate, data, htmlData{
		Intro:  "Use the code below to confirm " + data.NewEmail + " as the new address of your " + data.AppName + " account.",
		Footer: "If you did not ask for this change, ignore this email; the address stays unchanged.",
	})
}

func render(text *texttemplate.Template, data Data, html htmlData) (Message, error) {
	var textBody strings.Builder
	if err := text.Execute(&textBody, data); err != nil {
		return Message{}, err
	}

	html.Name = data.Name
	html.Code = data.Code
	html.Minutes = data.Minutes

	var htmlBody strings.Builder
	if err := htmlTemplate.Execute(&htmlBody, html); err != nil {
		return Message{}, err
	}

	return Message{Subject: Subject, Text: textBody.String(), HTML: htmlBody.String()}, nil
}
