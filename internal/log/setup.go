package log

import (
	"cloud.google.com/go/compute/metadata"
	"log/slog"
	"os"
)

// LevelCritical is an extra log level supported by Cloud Logging.
const LevelCritical = slog.Level(12)

// Set up structured logging
func init() {
	if metadata.OnGCE() {
		slog.SetDefault(slog.New(newHandler()))
	}
}

func newHandler() slog.Handler {
	return slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.MessageKey {
				a.Key = "message"
			} else if a.Key == slog.SourceKey {
				a.Key = "logging.googleapis.com/sourceLocation"
			} else if a.Key == slog.LevelKey {
				a.Key = "severity"
				level := a.Value.Any().(slog.Level)
				if level == LevelCritical {
					a.Value = slog.StringValue("CRITICAL")
				}
			}
			return a
		},
	})
}
