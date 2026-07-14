package cli

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"time"

	httpserver "github.com/gomatic/go-httpserver"

	"github.com/uplang/tsvsheet.go/internal/constants"
	"github.com/uplang/tsvsheet.go/internal/serve"
	"github.com/uplang/tsvsheet.go/internal/session"
	"github.com/uplang/tsvsheet.go/internal/sheet"
)

// shutdownTimeout bounds graceful shutdown when serve is interrupted.
const shutdownTimeout = 5 * time.Second

// filePerm is the mode for a saved worksheet file.
const filePerm = 0o600

// runServe loads the worksheet into a session, hosts it over HTTP, and serves
// until ctx is cancelled (Ctrl-C). Both template and data must be files — serve
// saves edits back to them, so stdin is not a valid source.
func runServe(ctx context.Context, cfg serveConfig) error {
	server, err := loadServer(cfg.template, cfg.data)
	if err != nil {
		return err
	}
	http := httpserver.New(slog.Default(), httpserver.Host(cfg.host), httpserver.Port(cfg.port), server.Handler())
	slog.Info("serving worksheet", "url", "http://"+http.Addr())
	return http.Serve(ctx, shutdownTimeout)
}

// loadServer reads the worksheet files into a session and builds the HTTP
// server with a saver that writes edits back to those files.
func loadServer(template, data sourcePath) (*serve.Server, error) {
	sess, persist, err := loadEditable(template, data)
	if err != nil {
		return nil, err
	}
	return serve.NewServer(sess, persist), nil
}

// loadSession reads both files and builds a session.
func loadSession(template, data sourcePath) (*session.Session, error) {
	templateBytes, err := os.ReadFile(string(template))
	if err != nil {
		return nil, constants.ErrOpenFile.With(err, string(template))
	}
	dataBytes, err := os.ReadFile(string(data))
	if err != nil {
		return nil, constants.ErrOpenFile.With(err, string(data))
	}
	grid, err := sheet.ReadTSV(bytes.NewReader(dataBytes))
	if err != nil {
		return nil, err
	}
	return session.New(templateBytes, grid)
}

// saver builds the persist function: it writes the session's current template
// and data back to their source files. The plain func() error is assignable to
// both serve.Saver and tui.Saver.
func saver(sess *session.Session, template, data sourcePath) func() error {
	return func() error {
		if err := os.WriteFile(string(template), sess.TemplateText(), filePerm); err != nil {
			return constants.ErrWriteFile.With(err, string(template))
		}
		if err := os.WriteFile(string(data), sess.DataTSV(), filePerm); err != nil {
			return constants.ErrWriteFile.With(err, string(data))
		}
		return nil
	}
}
