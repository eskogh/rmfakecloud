package email

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// No message is sent: the fake server rejects STARTTLS after verifying SMTP order.
func TestSTARTTLSBeginsWithSMTPAndCustomHello(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	result := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			result <- err
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		fmt.Fprint(conn, "220 test SMTP\r\n")
		reader := bufio.NewReader(conn)
		line, err := reader.ReadString('\n')
		if err != nil {
			result <- err
			return
		}
		if line != "EHLO cloud.example.com\r\n" {
			result <- fmt.Errorf("expected custom EHLO before TLS, got %q", line)
			return
		}
		fmt.Fprint(conn, "250-test\r\n250 STARTTLS\r\n")
		line, err = reader.ReadString('\n')
		if err != nil {
			result <- err
			return
		}
		if line != "STARTTLS\r\n" {
			result <- fmt.Errorf("expected STARTTLS, got %q", line)
			return
		}
		fmt.Fprint(conn, "454 test deliberately refuses TLS\r\n")
		result <- nil
	}()
	builder := &Builder{}
	err = builder.Send(&SMTPConfig{Server: listener.Addr().String(), StartTLS: true, Helo: "cloud.example.com"})
	if err == nil || !strings.Contains(err.Error(), "454") {
		t.Fatalf("expected test TLS refusal: %v", err)
	}
	if err = <-result; err != nil {
		t.Fatal(err)
	}
}
