package email

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestSendDiagnostics(t *testing.T) {
	for _, failure := range []string{"", "AUTH", "MAIL", "RCPT", "DATA", "acceptance"} {
		t.Run(failure, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(3 * time.Second))
				fmt.Fprint(conn, "220 test\r\n")
				scanner := bufio.NewScanner(conn)
				data := false
				for scanner.Scan() {
					line := scanner.Text()
					if data {
						if line != "." {
							continue
						}
						data = false
						if failure == "acceptance" {
							fmt.Fprint(conn, "550 quota exceeded\r\n")
						} else {
							fmt.Fprint(conn, "250 queued\r\n")
						}
						continue
					}
					command := strings.Fields(line)[0]
					if command == failure {
						fmt.Fprint(conn, "535 rejected secret-password\r\n")
						continue
					}
					switch command {
					case "EHLO":
						fmt.Fprint(conn, "250-test\r\n250 AUTH PLAIN\r\n")
					case "AUTH":
						fmt.Fprint(conn, "235 authenticated\r\n")
					case "DATA":
						fmt.Fprint(conn, "354 send message\r\n")
						data = true
					case "QUIT":
						return // acceptance must succeed even if QUIT loses its reply
					default:
						fmt.Fprint(conn, "250 ok\r\n")
					}
				}
			}()
			b := &Builder{From: &mail.Address{Address: "sender@example.com"}, To: []*mail.Address{{Address: "recipient@example.com"}}, Body: "test", Subject: "test"}
			cfg := &SMTPConfig{Server: listener.Addr().String(), NoTLS: true, Username: "user", Password: "secret-password"}
			err = b.Send(cfg)
			if failure == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				stage := map[string]string{"AUTH": "authentication", "MAIL": "sender", "RCPT": "recipient", "DATA": "message transfer", "acceptance": "message acceptance"}[failure]
				if err == nil || !strings.Contains(err.Error(), stage) || strings.Contains(err.Error(), cfg.Password) {
					t.Fatalf("unexpected diagnostic: %v", err)
				}
			}
			<-done
		})
	}
}

func TestSendContextStopsStalledGreeting(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(time.Second))
		var b [1]byte
		conn.Read(b[:])
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = (&Builder{}).SendContext(ctx, &SMTPConfig{Server: listener.Addr().String(), NoTLS: true})
	if err == nil || !strings.Contains(err.Error(), "server greeting") {
		t.Fatalf("unexpected error: %v", err)
	}
	<-done
}

func TestSMTPErrorRedactsCredentials(t *testing.T) {
	cfg := &SMTPConfig{Username: "user", Password: "private-password"}
	encoded := base64.StdEncoding.EncodeToString([]byte("\x00" + cfg.Username + "\x00" + cfg.Password))
	encodedPassword := base64.StdEncoding.EncodeToString([]byte(cfg.Password))
	detail := safeSMTPError(errors.New("535 "+encoded+" "+encodedPassword+" "+cfg.Password+"\r\nrejected"), cfg)
	for _, secret := range []string{cfg.Password, encoded, encodedPassword, "\r", "\n"} {
		if strings.Contains(detail, secret) {
			t.Fatalf("unsafe diagnostic: %q", detail)
		}
	}
	if !strings.Contains(detail, "535") {
		t.Fatal("lost SMTP error code")
	}
}
