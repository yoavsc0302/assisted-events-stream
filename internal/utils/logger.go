package utils

import (
	"os"

	"github.com/sirupsen/logrus"
)

func NewLogger() *logrus.Logger {
	logger := logrus.New()
	logger.Out = os.Stdout
	logger.SetFormatter(&logrus.JSONFormatter{})
	lvl, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		lvl = "debug"
	}
	level, err := logrus.ParseLevel(lvl)
	if err != nil {
		level = logrus.DebugLevel
	}
	logger.Level = level
	return logger
}
