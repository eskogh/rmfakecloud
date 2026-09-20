package email

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// MaxLineLength is the maximum line length per RFC 2045
	MaxLineLength = 76
	delimeter     = "**=myohmy689407924327898338383"
	smtpLog       = "[smtp] "
)

// SMTPConfig smtp configuration
type SMTPConfig struct {
	Server       string
	Username     string
	Password     string
	FromOverride *mail.Address
	Helo         string
	InsecureTLS  bool
	NoTLS        bool
	StartTLS     bool
}

// Builder builds emails
type Builder struct {
	From    *mail.Address
	To      []*mail.Address
	ReplyTo *mail.Address
	Body    string
	Subject string

	attachments []emailAttachment
}

type emailAttachment struct {
	filename    string
	contentType string
	data        io.Reader
}

// TrimAddresses workaround for go < 1.15
func TrimAddresses(address string) string {
	return strings.Trim(strings.Trim(address, " "), ",")
}

// AddFile adds a file attachment
func (b *Builder) AddFile(name string, data io.Reader, contentType string) {
	log.Debugln("Adding file: ", name, " contentType: ", contentType)
	if contentType == "" {
		log.Warnln("no contentType, setting to binary")
		contentType = "application/octet-stream"
	}
	attachment := emailAttachment{
		contentType: contentType,
		filename:    filepath.Base(name),
		data:        data,
	}
	b.attachments = append(b.attachments, attachment)
}

// WriteAttachments streams the attachments
func (b *Builder) WriteAttachments(w io.Writer) (err error) {
	for _, attachment := range b.attachments {
		log.Debugln("File attachment: ", attachment.filename)

		fileHeader := fmt.Sprintf("\r\n--%s\r\n", delimeter)
		fileHeader += "Content-Type: " + attachment.contentType + "; charset=\"utf-8\"\r\n"
		fileHeader += "Content-Transfer-Encoding: base64\r\n"
		fileHeader += "Content-Disposition: attachment;filename*=utf-8''" + url.PathEscape(attachment.filename) + "\r\n\r\n"
		_, err = w.Write([]byte(fileHeader))
		if err != nil {
			return err
		}

		splittingEncoder := &SplittingWritter{
			innerWriter:    w,
			maxLineLength:  MaxLineLength,
			lineTerminator: "\r\n",
		}
		base64Encoder := base64.NewEncoder(base64.StdEncoding, splittingEncoder)
		_, err := io.Copy(base64Encoder, attachment.data)

		if err != nil {
			return err
		}
		base64Encoder.Close()
	}
	return nil
}

func utf8encode(s string) string {
	return mime.QEncoding.Encode("utf-8", s)
}

// Send sends the email
func (b *Builder) Send(cfg *SMTPConfig) (err error) {
	return b.SendContext(context.Background(), cfg)
}

// SendContext sends with a bounded lifetime and reports the failing SMTP stage.
func (b *Builder) SendContext(ctx context.Context, cfg *SMTPConfig) (err error) {
	stage := "configuration"
	defer func() {
		if err != nil {
			if ctx.Err() != nil {
				err = ctx.Err()
			}
			err = fmt.Errorf("SMTP %s: %s", stage, safeSMTPError(err, cfg))
		}
	}()
	if cfg == nil {
		return errors.New("no smtp config")
	}

	helo := ResolveHelo(cfg.Helo, "", "", nil, false)
	if helo == "" || strings.ContainsAny(helo, "\r\n") {
		return errors.New("configure an SMTP HELO hostname; no valid hostname is available")
	}

	host, _, err := net.SplitHostPort(cfg.Server)
	if err != nil {
		return err
	}

	stage = "connection (check DNS, host, port, and firewall)"
	var conn net.Conn

	tlsconfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureTLS,
		ServerName:         host,
	}

	dialer := &net.Dialer{Timeout: 15 * time.Second}
	if cfg.NoTLS || cfg.StartTLS {
		conn, err = dialer.DialContext(ctx, "tcp", cfg.Server)
	} else {
		stage = "TLS connection (check port and certificate)"
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsconfig}).DialContext(ctx, "tcp", cfg.Server)
	}

	if err != nil {
		return err
	}

	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	stage = "server greeting"
	if err = conn.SetDeadline(time.Now().Add(2 * time.Minute)); err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}

	stage = "EHLO (check HELO hostname)"
	if err = c.Hello(helo); err != nil {
		return err
	}
	if cfg.StartTLS {
		stage = "STARTTLS (check encryption mode and certificate)"
		if err = c.StartTLS(tlsconfig); err != nil {
			return err
		}
	}

	if cfg.Username != "" {
		stage = "authentication (check username, password, and provider authentication policy)"
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, host)
		if err = c.Auth(auth); err != nil {
			return err
		}
	}

	stage = "sender (check allowed sender address and relay IP policy)"
	if err = c.Mail(b.From.Address); err != nil {
		return err
	}

	stage = "recipient (check recipient address and relay permissions)"
	for _, addr := range b.To {
		if err = c.Rcpt(addr.Address); err != nil {
			return err
		}
	}

	stage = "message transfer"
	w, err := c.Data()
	if err != nil {
		return err
	}

	toList := make([]string, 0)
	for _, toAddr := range b.To {
		toList = append(toList, toAddr.String())
	}
	to := strings.Join(toList, ", ")

	msgBuilder := strings.Builder{}
	//basic email headers
	msgBuilder.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msgBuilder.WriteString(fmt.Sprintf("From: %s\r\n", utf8encode(b.From.String())))
	msgBuilder.WriteString(fmt.Sprintf("To: %s\r\n", utf8encode(to)))
	if b.ReplyTo != nil {
		msgBuilder.WriteString(fmt.Sprintf("Reply-To: %s\r\n", utf8encode(b.ReplyTo.String())))
	}
	msgBuilder.WriteString(fmt.Sprintf("Subject: %s\r\n", utf8encode(b.Subject)))
	msgBuilder.WriteString("MIME-Version: 1.0\r\n")
	msgBuilder.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", delimeter))
	msgBuilder.WriteString(fmt.Sprintf("\r\n--%s\r\n", delimeter))
	msgBuilder.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	msgBuilder.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	msgBuilder.WriteString("Content-Disposition: inline\r\n")
	msgBuilder.WriteString("\r\n")
	msgBuilder.WriteString(b.Body)

	msg := msgBuilder.String()

	log.Debug("mime msg:\n", msg)

	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}

	err = b.WriteAttachments(w)
	if err != nil {
		return err
	}

	// Add last boundary delimeter, with trailing -- according to RFC 1341
	lastBoundary := fmt.Sprintf("\r\n--%s--\r\n", delimeter)
	_, err = w.Write([]byte(lastBoundary))
	if err != nil {
		return err
	}

	stage = "message acceptance (check provider policy or quota)"
	err = w.Close()
	if err != nil {
		return err
	}

	// DATA acceptance is success even if the server closes before QUIT.
	_ = c.Quit()
	return nil
}

// SplittingWritter writes a stream and inserts a terminator
type SplittingWritter struct {
	innerWriter       io.Writer
	currentLineLength int
	maxLineLength     int
	lineTerminator    string
}

func (w *SplittingWritter) Write(p []byte) (n int, err error) {
	length := len(p)
	total := 0
	for to, from := 0, 0; from < length; from = to {
		delta := w.maxLineLength - w.currentLineLength

		to = from + delta
		if to > length {
			to = length
			delta = length - from
		}

		n, err = w.innerWriter.Write(p[from:to])
		total += n
		if err != nil {
			return total, err
		}

		w.currentLineLength += delta

		if w.currentLineLength == w.maxLineLength {
			n, err = w.innerWriter.Write([]byte(w.lineTerminator))
			total += n
			if err != nil {
				return total, err
			}
			w.currentLineLength = 0
		}
	}

	return total, nil
}

// Avoid disclosing credentials even if an SMTP server echoes authentication data.
func safeSMTPError(err error, cfg *SMTPConfig) string {
	detail := err.Error()
	if cfg != nil && cfg.Password != "" {
		for _, secret := range []string{base64.StdEncoding.EncodeToString([]byte("\x00" + cfg.Username + "\x00" + cfg.Password)), base64.StdEncoding.EncodeToString([]byte(cfg.Password)), cfg.Password} {
			detail = strings.ReplaceAll(detail, secret, "[redacted]")
		}
	}
	detail = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, detail)
	if len(detail) > 1000 {
		detail = detail[:1000]
	}
	return detail
}
