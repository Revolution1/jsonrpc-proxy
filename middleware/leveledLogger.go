package middleware

import log "github.com/sirupsen/logrus"

type LeveledLogger struct {
	Level log.Level
}

func (l LeveledLogger) Printf(format string, args ...interface{}) {
	log.StandardLogger().Logf(l.Level, format, args...)
}
