package logger

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var ServerLogger zerolog.Logger

func InitLogger(logLevel zerolog.Level) {

	// default api logger (json)
	log.Logger = zerolog.New(os.Stdout).
		Output(os.Stdout).
		With().
		Timestamp().
		Caller().
		Logger()

	// server logs
	ServerLogger = zerolog.New(os.Stdout).
		Output(zerolog.ConsoleWriter{Out: os.Stdout}).
		With().
		Timestamp().
		Caller().
		Logger() //std log

	if logLevel == zerolog.DebugLevel {
		log.Logger = ServerLogger //user standard console log when in debug mode
	}
}
