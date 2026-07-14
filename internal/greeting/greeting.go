// Package greeting composes greeting messages from a salutation and a recipient.
//
// It provides small, pure building blocks — composition, casing, emphasis, and
// repetition — that callers combine into a final message. The package holds no
// CLI or orchestration logic; it is the reusable implementation behind the
// greet command's domain. Anything here could be imported by another domain
// that needs to build greetings.
package greeting

import "strings"

type (
	// Salutation is the greeting word, e.g. "Hello".
	Salutation string
	// Recipient is the entity being greeted.
	Recipient string
	// Message is a composed greeting line.
	Message string
	// Marks is a count of trailing exclamation marks used for emphasis.
	Marks int
	// Count is the number of times a message is repeated.
	Count int
)

// Compose builds the base greeting "<salutation>, <recipient>!".
func Compose(salutation Salutation, recipient Recipient) Message {
	return Message(string(salutation) + ", " + string(recipient) + "!")
}

// Uppercase returns the message converted to uppercase.
func Uppercase(message Message) Message {
	return Message(strings.ToUpper(string(message)))
}

// Emphasize appends marks exclamation marks to the message.
func Emphasize(message Message, marks Marks) Message {
	return Message(string(message) + strings.Repeat("!", int(marks)))
}

// Repeat joins count copies of the message with newlines.
func Repeat(message Message, count Count) Message {
	lines := make([]string, count)
	for index := range lines {
		lines[index] = string(message)
	}
	return Message(strings.Join(lines, "\n"))
}
