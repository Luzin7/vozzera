package smtp

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSend(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	received := make(chan string, 1)
	go serve(ln, received)

	addr := ln.Addr().(*net.TCPAddr)
	m, err := NewSmtpMailer(Config{
		Host:     addr.IP.String(),
		Port:     addr.Port,
		User:     "user",
		Password: "pass",
		From:     "from@example.com",
	})
	if err != nil {
		t.Fatalf("NewSmtpMailer() erro inesperado: %v", err)
	}

	if err := m.Send("to@example.com", "Assunto", "<p>oi</p>"); err != nil {
		t.Fatalf("Send() erro inesperado: %v", err)
	}

	select {
	case msg := <-received:
		if !strings.Contains(msg, "Subject: Assunto") {
			t.Errorf("subject ausente: %q", msg)
		}
		if !strings.Contains(msg, "To: to@example.com") {
			t.Errorf("destinatário ausente: %q", msg)
		}
		if !strings.Contains(msg, "<p>oi</p>") {
			t.Errorf("corpo ausente: %q", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("nenhuma mensagem recebida pelo servidor")
	}
}

func TestNewSmtpMailerInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "host vazio", cfg: Config{Port: 587, User: "u", Password: "p", From: "f@example.com"}},
		{name: "porta zero", cfg: Config{Host: "smtp.example.com", User: "u", Password: "p", From: "f@example.com"}},
		{name: "user vazio", cfg: Config{Host: "smtp.example.com", Port: 587, Password: "p", From: "f@example.com"}},
		{name: "senha vazia", cfg: Config{Host: "smtp.example.com", Port: 587, User: "u", From: "f@example.com"}},
		{name: "remetente vazio", cfg: Config{Host: "smtp.example.com", Port: 587, User: "u", Password: "p"}},
		{name: "config vazia", cfg: Config{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewSmtpMailer(tt.cfg); err == nil {
				t.Error("NewSmtpMailer() esperava erro")
			}
		})
	}
}

func serve(ln net.Listener, received chan<- string) {
	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	r := bufio.NewReader(conn)
	reply := func(line string) {
		fmt.Fprintf(conn, "%s\r\n", line)
	}

	reply("220 localhost ESMTP")

	var data strings.Builder
	inData := false

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		switch {
		case inData:
			if line == "." {
				inData = false
				received <- data.String()
				reply("250 queued")
				continue
			}
			data.WriteString(line)
			data.WriteString("\n")
		case strings.HasPrefix(line, "EHLO"):
			reply("250-localhost")
			reply("250-AUTH PLAIN")
			reply("250 8BITMIME")
		case strings.HasPrefix(line, "AUTH"):
			reply("235 ok")
		case strings.HasPrefix(line, "MAIL FROM"):
			reply("250 ok")
		case strings.HasPrefix(line, "RCPT TO"):
			reply("250 ok")
		case strings.HasPrefix(line, "DATA"):
			inData = true
			reply("354 go ahead")
		case strings.HasPrefix(line, "QUIT"):
			reply("221 bye")
			return
		}
	}
}
