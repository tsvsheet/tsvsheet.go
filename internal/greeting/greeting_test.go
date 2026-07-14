package greeting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompose(t *testing.T) {
	t.Parallel()
	assert.New(t).Equal(Message("Hello, World!"), Compose("Hello", "World"))
}

func TestUppercase(t *testing.T) {
	t.Parallel()
	assert.New(t).Equal(Message("HELLO, WORLD!"), Uppercase("Hello, World!"))
}

func TestEmphasize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		message Message
		want    Message
		marks   Marks
	}{
		{name: "no marks", message: "Hi!", marks: 0, want: "Hi!"},
		{name: "two marks", message: "Hi!", marks: 2, want: "Hi!!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.New(t).Equal(tt.want, Emphasize(tt.message, tt.marks))
		})
	}
}

func TestRepeat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		message Message
		want    Message
		count   Count
	}{
		{name: "once", message: "Hi!", count: 1, want: "Hi!"},
		{name: "three times", message: "Hi!", count: 3, want: "Hi!\nHi!\nHi!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.New(t).Equal(tt.want, Repeat(tt.message, tt.count))
		})
	}
}
