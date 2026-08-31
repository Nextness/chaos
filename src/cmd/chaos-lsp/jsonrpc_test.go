package main

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
	"testing"
)

func TestReadWriteMessageRoundTrip(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	var buf bytes.Buffer
	if err := writeMessage(&buf, body); err != nil {
		t.Fatalf("writeMessage: %v", err)
	}
	got, err := readMessage(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("readMessage: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("round-trip body = %q, want %q", got, body)
	}
}

func TestReadMessageMultiMessageStream(t *testing.T) {
	var buf bytes.Buffer
	for _, body := range []string{`{"a":1}`, `{"b":2}`, `{"c":3}`} {
		if err := writeMessage(&buf, []byte(body)); err != nil {
			t.Fatalf("writeMessage: %v", err)
		}
	}
	r := bufio.NewReader(&buf)
	for i, want := range []string{`{"a":1}`, `{"b":2}`, `{"c":3}`} {
		got, err := readMessage(r)
		if err != nil {
			t.Fatalf("message %d: %v", i, err)
		}
		if string(got) != want {
			t.Errorf("message %d = %q, want %q", i, got, want)
		}
	}
}

func TestReadMessageCaseInsensitiveHeader(t *testing.T) {
	input := "content-length: 5\r\n\r\nhello"
	got, err := readMessage(bufio.NewReader(strings.NewReader(input)))
	if err != nil {
		t.Fatalf("readMessage: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("body = %q, want %q", got, "hello")
	}
}

func TestReadMessageZeroLengthBody(t *testing.T) {
	input := "Content-Length: 0\r\n\r\n"
	got, err := readMessage(bufio.NewReader(strings.NewReader(input)))
	if err != nil {
		t.Fatalf("readMessage: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("body = %q, want empty", got)
	}
}

func TestReadMessageMalformedHeader(t *testing.T) {
	input := "NotAHeader\r\n\r\n"
	if _, err := readMessage(bufio.NewReader(strings.NewReader(input))); err == nil {
		t.Error("expected error for malformed header, got nil")
	}
}

func TestReadMessageMissingContentLength(t *testing.T) {
	input := "Content-Type: application/json\r\n\r\n"
	if _, err := readMessage(bufio.NewReader(strings.NewReader(input))); err == nil {
		t.Error("expected error for missing content-length, got nil")
	}
}

func TestReadMessageInvalidContentLength(t *testing.T) {
	input := "Content-Length: abc\r\n\r\n"
	if _, err := readMessage(bufio.NewReader(strings.NewReader(input))); err == nil {
		t.Error("expected error for invalid content-length, got nil")
	}
}

func TestReadMessageRejectsDuplicateContentLength(t *testing.T) {
	input := "Content-Length: 0\r\ncontent-length: 0\r\n\r\n"
	if _, err := readMessage(bufio.NewReader(strings.NewReader(input))); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate content-length error = %v", err)
	}
}

func TestReadMessageRejectsOversizedBody(t *testing.T) {
	input := "Content-Length: " + strconv.Itoa(maxBodyBytes+1) + "\r\n\r\n"
	if _, err := readMessage(bufio.NewReader(strings.NewReader(input))); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized-body error = %v", err)
	}
}

func TestReadMessageRejectsOversizedHeaderLine(t *testing.T) {
	input := "X-Long: " + strings.Repeat("x", maxHeaderLine) + "\r\nContent-Length: 0\r\n\r\n"
	if _, err := readMessage(bufio.NewReader(strings.NewReader(input))); err == nil || !strings.Contains(err.Error(), "header line") {
		t.Fatalf("oversized-header error = %v", err)
	}
}

func TestReadMessageRejectsUnterminatedOversizedHeaderLine(t *testing.T) {
	input := strings.Repeat("x", maxHeaderLine+1)
	if _, err := readMessage(bufio.NewReader(strings.NewReader(input))); err == nil || !strings.Contains(err.Error(), "header line") {
		t.Fatalf("unterminated oversized-header error = %v", err)
	}
}

func TestReadMessageRejectsTruncatedBody(t *testing.T) {
	input := "Content-Length: 5\r\n\r\nabc"
	if _, err := readMessage(bufio.NewReader(strings.NewReader(input))); err == nil {
		t.Fatal("expected truncated body error")
	}
}
