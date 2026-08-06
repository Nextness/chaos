package main

import (
	"bufio"
	"bytes"
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
