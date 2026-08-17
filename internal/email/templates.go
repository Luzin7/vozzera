package email

import (
	"html"
	"net/url"
)

func PasswordReset(appURL, token string) (subject, body string) {
	link := html.EscapeString(appURL + "/reset?token=" + url.QueryEscape(token))
	content := "<p>Olá,</p>" +
		"<p>Recebemos um pedido para redefinir a sua senha na Vozzera.</p>" +
		"<p>Para continuar, acesse o link abaixo:</p>" +
		`<p><a href="` + link + `">Redefinir senha</a></p>` +
		"<p>Se você não solicitou, pode ignorar este email.</p>"
	return "Redefinição de senha — Vozzera", layout(content)
}

func Welcome(appURL, username string) (subject, body string) {
	content := "<p>Olá, <strong>" + html.EscapeString(username) + "</strong>,</p>" +
		"<p>Seja bem-vindo(a) à Vozzera! Estamos felizes em ter você por aqui.</p>" +
		`<p><a href="` + html.EscapeString(appURL) + `">Acessar a plataforma</a></p>`
	return "Bem-vindo à Vozzera", layout(content)
}

func layout(content string) string {
	return `<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:Arial,Helvetica,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;padding:24px 0;">
<tr>
<td align="center">
<table role="presentation" width="600" cellpadding="0" cellspacing="0" style="max-width:600px;width:100%;background:#ffffff;border-radius:8px;">
<tr>
<td style="background:#111827;padding:20px 24px;text-align:center;">
<span style="color:#ffffff;font-size:18px;font-weight:bold;">Vozzera</span>
</td>
</tr>
<tr>
<td style="padding:32px 24px;color:#111827;font-size:15px;line-height:1.6;">
` + content + `
</td>
</tr>
</table>
</td>
</tr>
</table>
</body>
</html>`
}
