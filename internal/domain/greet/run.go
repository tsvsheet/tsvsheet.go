package greet

import (
	"context"
	"log/slog"

	"github.com/gomatic/template.cli/internal/constants"
	"github.com/gomatic/template.cli/internal/domain"
	"github.com/gomatic/template.cli/internal/greeting"
)

const (
	defaultSalutation = "Hello" // defaultSalutation applies when --greeting is empty.
	enthusiasmMarks   = 2       // enthusiasmMarks is the emphasis added by --enthusiast.
	minimumRepeat     = 1       // minimumRepeat is the floor for --repeat.
)

// Result is the outcome of the greet command.
type Result struct {
	Message greeting.Message `json:"message"`
}

// Run composes a greeting for the recipient named in args, applying the
// transformations selected in cfg. It orchestrates the greeting package and
// holds no presentation logic.
func Run(_ context.Context, logger *slog.Logger, cfg Config, args ...domain.Argument) (Result, error) {
	recipient, err := recipientFrom(args)
	if err != nil {
		return Result{}, err
	}

	message := compose(cfg, recipient)

	logger.Info("Greeting generated.", "recipient", recipient, "repeat", repeatOrDefault(cfg.Repeat))
	return Result{Message: message}, nil
}

// recipientFrom extracts and validates the recipient from the positional args.
func recipientFrom(args []string) (greeting.Recipient, error) {
	if len(args) < 1 {
		return "", constants.ErrMissingArgument.With(nil, "name")
	}
	if args[0] == "" {
		return "", constants.ErrInvalidName.With(nil, "name cannot be empty")
	}
	return greeting.Recipient(args[0]), nil
}

// compose builds the final message by applying the configured transformations.
func compose(cfg Config, recipient greeting.Recipient) greeting.Message {
	message := greeting.Compose(salutationOrDefault(cfg.Greeting), recipient)
	if bool(cfg.UppercaseEnabled) {
		message = greeting.Uppercase(message)
	}
	if bool(cfg.EnthusiastEnabled) {
		message = greeting.Emphasize(message, enthusiasmMarks)
	}
	return greeting.Repeat(message, repeatOrDefault(cfg.Repeat))
}

// salutationOrDefault returns the configured salutation or the default.
func salutationOrDefault(configured salutation) greeting.Salutation {
	if configured == "" {
		return defaultSalutation
	}
	return greeting.Salutation(configured)
}

// repeatOrDefault clamps the repeat count to at least the minimum.
func repeatOrDefault(configured repeatCount) greeting.Count {
	if configured < minimumRepeat {
		return minimumRepeat
	}
	return greeting.Count(configured)
}
