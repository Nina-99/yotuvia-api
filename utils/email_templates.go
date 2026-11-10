package utils

import (
	"fmt"
	"html"
	"net/url"
	"os"
	"strings"
)

// The backend does not know the recipient's locale (registration/login carry no
// language, and no Accept-Language is persisted per account), and the primary
// market is Spanish-speaking, so the transactional emails are written in
// neutral/professional Spanish. When per-user locale exists, these builders can
// branch on it; the call sites already pass everything they need.
//
// Visual identity: the Yotuvia gold accent (#CFB53B) is taken from the mobile
// design tokens (src/constants/theme.ts, goldYottu) and kept simple — a single
// gold button on a light card, table-based so it renders in most mail clients.

// EmailContent is a rendered transactional email without a recipient.
type EmailContent struct {
	Subject string
	Text    string
	HTML    string
}

// DeepLinkBaseURL returns the app-scheme base for email links, configured with
// APP_DEEP_LINK_BASE_URL (default "yotuvia://"). The mobile app registers both
// "yotuvia" and "yottumobile" schemes; "yotuvia" is the canonical external one.
//
// NOTE: an app-scheme link only works when the app is installed. A hosted web
// fallback (e.g. https://yotuvia.app/verify?token=...) is REQUIRED for users
// without the app and is intentionally out of scope here; pointing this env var
// at such a base later requires no code change beyond the destination routes.
func DeepLinkBaseURL() string {
	base := strings.TrimSpace(os.Getenv("APP_DEEP_LINK_BASE_URL"))
	if base == "" {
		return "yotuvia://"
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base
}

// VerificationLink builds the deep link a user taps to verify their email.
func VerificationLink(rawToken string) string {
	return buildDeepLink("verify-email", rawToken)
}

// PasswordResetLink builds the deep link a user taps to reset their password.
func PasswordResetLink(rawToken string) string {
	return buildDeepLink("reset-password", rawToken)
}

func buildDeepLink(path, rawToken string) string {
	return fmt.Sprintf("%s%s?token=%s", DeepLinkBaseURL(), path, url.QueryEscape(rawToken))
}

// VerificationEmail renders the email-verification message.
func VerificationEmail(firstName, link string) EmailContent {
	name := displayName(firstName)
	subject := "Verifica tu correo en Yotuvia"

	text := fmt.Sprintf(`¡Hola %s!

Confirma que este correo es tuyo para completar la verificación de tu cuenta en Yotuvia.

Abre este enlace para verificar tu correo:
%s

Por seguridad, el enlace vence en 24 horas y solo puede usarse una vez.

Si no creaste una cuenta en Yotuvia, puedes ignorar este mensaje.

— El equipo de Yotuvia`, name, link)

	return EmailContent{
		Subject: subject,
		Text:    text,
		HTML:    emailLayout(contextVerification, name, link),
	}
}

// PasswordResetEmail renders the password-recovery message.
func PasswordResetEmail(firstName, link string) EmailContent {
	name := displayName(firstName)
	subject := "Restablece tu contraseña de Yotuvia"

	text := fmt.Sprintf(`¡Hola %s!

Recibimos una solicitud para restablecer la contraseña de tu cuenta en Yotuvia.

Abre este enlace para crear una contraseña nueva:
%s

Por seguridad, el enlace vence en 1 hora y solo puede usarse una vez. Al usarlo, se cerrarán todas tus sesiones activas.

Si no solicitaste este cambio, puedes ignorar este mensaje: tu contraseña no cambiará.

— El equipo de Yotuvia`, name, link)

	return EmailContent{
		Subject: subject,
		Text:    text,
		HTML:    emailLayout(contextPasswordReset, name, link),
	}
}

type emailContext struct {
	heading      string
	intro        string
	buttonLabel  string
	expiryNote   string
	securityNote string
}

var (
	contextVerification = emailContext{
		heading:      "Verifica tu correo",
		intro:        "Confirma que este correo es tuyo para completar la verificación de tu cuenta en Yotuvia.",
		buttonLabel:  "Verificar mi correo",
		expiryNote:   "El enlace vence en 24 horas y solo puede usarse una vez.",
		securityNote: "Si no creaste una cuenta en Yotuvia, puedes ignorar este mensaje.",
	}
	contextPasswordReset = emailContext{
		heading:      "Restablece tu contraseña",
		intro:        "Recibimos una solicitud para restablecer la contraseña de tu cuenta en Yotuvia.",
		buttonLabel:  "Crear contraseña nueva",
		expiryNote:   "El enlace vence en 1 hora y solo puede usarse una vez.",
		securityNote: "Si no solicitaste este cambio, ignora este mensaje: tu contraseña no cambiará.",
	}
)

// displayName falls back to a neutral greeting when the name is empty.
func displayName(firstName string) string {
	if strings.TrimSpace(firstName) == "" {
		return "hola"
	}
	return strings.TrimSpace(firstName)
}

// emailLayout renders the shared table-based HTML shell with the Yotuvia gold
// accent. All interpolated values are HTML-escaped.
func emailLayout(ctx emailContext, name, link string) string {
	e := html.EscapeString
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f5;font-family:Arial,Helvetica,sans-serif;color:#1a1a2e;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f5;">
    <tr>
      <td align="center" style="padding:24px 12px;">
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:480px;background-color:#ffffff;border-radius:12px;overflow:hidden;">
          <tr>
            <td style="background-color:#121212;padding:20px 28px;">
              <span style="font-size:20px;font-weight:bold;color:#CFB53B;letter-spacing:1px;">YOTUVIA</span>
            </td>
          </tr>
          <tr>
            <td style="padding:28px;">
              <h1 style="margin:0 0 12px;font-size:20px;color:#1a1a2e;">%s</h1>
              <p style="margin:0 0 8px;font-size:15px;line-height:22px;">¡Hola %s!</p>
              <p style="margin:0 0 20px;font-size:15px;line-height:22px;color:#444;">%s</p>
              <table role="presentation" cellpadding="0" cellspacing="0">
                <tr>
                  <td style="border-radius:8px;background-color:#CFB53B;">
                    <a href="%s" style="display:inline-block;padding:12px 24px;font-size:15px;font-weight:bold;color:#121212;text-decoration:none;">%s</a>
                  </td>
                </tr>
              </table>
              <p style="margin:24px 0 0;font-size:13px;line-height:20px;color:#666;">Si el botón no funciona, copia y pega este enlace en tu navegador o ábrelo desde tu teléfono:</p>
              <p style="margin:8px 0 0;font-size:12px;line-height:18px;color:#8a8a8a;word-break:break-all;">%s</p>
            </td>
          </tr>
          <tr>
            <td style="padding:0 28px 24px;">
              <p style="margin:16px 0 0;font-size:13px;line-height:20px;color:#666;">%s</p>
              <p style="margin:8px 0 0;font-size:13px;line-height:20px;color:#666;">%s</p>
            </td>
          </tr>
          <tr>
            <td style="border-top:1px solid #ececec;padding:16px 28px;">
              <p style="margin:0;font-size:12px;color:#9a9a9a;">Este es un mensaje automático, por favor no respondas a este correo.</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, e(ctx.heading), e(ctx.heading), e(name), e(ctx.intro), e(link), e(ctx.buttonLabel), e(link), e(ctx.expiryNote), e(ctx.securityNote))
}
