package mail

import (
	"fmt"
	"html"
)

// SignInLink is the magic-link email. Plain and short, matching the app's voice.
func SignInLink(to, link string) Message {
	return Message{
		To:      to,
		Subject: "Your rift sign-in link",
		Text: fmt.Sprintf("Open this link to sign in to rift:\n\n%s\n\n"+
			"It works once and expires in 15 minutes. If you didn't ask for it, you can ignore this email.\n", link),
		HTML: fmt.Sprintf(`<p>Open this link to sign in to rift:</p>`+
			`<p><a href="%[1]s">Sign in to rift</a></p>`+
			`<p style="color:#6c7086">It works once and expires in 15 minutes. If you didn't ask for it, you can ignore this email.</p>`,
			html.EscapeString(link)),
	}
}
