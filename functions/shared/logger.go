package shared

import (
	"os"

	"github.com/rs/zerolog"
)

var logger zerolog.Logger

func init() {
	logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

func LogInfo() *zerolog.Event {
	return logger.Info()
}

func LogError() *zerolog.Event {
	return logger.Error()
}

func LogWarn() *zerolog.Event {
	return logger.Warn()
}

func LogDebug() *zerolog.Event {
	return logger.Debug()
}
