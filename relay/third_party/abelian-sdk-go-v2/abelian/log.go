package abelian

import (
	"fmt"
	"github.com/jrick/logrotate/rotator"
	"github.com/pqabelian/abelian-sdk-go-v2/abelian/crypto"
	"github.com/pqabelian/abelian-sdk-go-v2/abelian/logger"
	"os"
	"path/filepath"
)

type logWriter struct{}

func (logWriter) Write(p []byte) (n int, err error) {
	logRotator.Write(p)
	return len(p), nil
}

var (
	backendLog = logger.NewBackend(logWriter{})

	// logRotator is one of the logging outputs.  It should be closed on
	// application shutdown.
	logRotator *rotator.Rotator

	cryptoLog = backendLog.Logger("CRYPTO")
	sdkLog    = backendLog.Logger("ABELIAN")
)

func init() {
	crypto.UseLogger(cryptoLog)
}

var subsystemLoggers = map[string]logger.Logger{
	"CRYPTO":  cryptoLog,
	"ABELIAN": sdkLog,
}

func initLogRotator(logFile string) {
	logDir, _ := filepath.Split(logFile)
	err := os.MkdirAll(logDir, 0700)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log directory: %v\n", err)
		os.Exit(1)
	}
	r, err := rotator.New(logFile, 10*1024, false, 30)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create file rotator: %v\n", err)
		os.Exit(1)
	}

	logRotator = r
}
func setLogLevel(subsystemID string, logLevel string) {
	// Ignore invalid subsystems.
	subLogger, ok := subsystemLoggers[subsystemID]
	if !ok {
		return
	}

	// Defaults to info if the log level is invalid.
	level, _ := logger.LevelFromString(logLevel)
	subLogger.SetLevel(level)
}
func setLogLevels(logLevel string) {
	// Configure all sub-systems with the new logging level.  Dynamically
	// create loggers as needed.
	for subsystemID := range subsystemLoggers {
		setLogLevel(subsystemID, logLevel)
	}
}

func init() {
	wd, err := os.Getwd()
	if err != nil {
		wd = "./.abelian/sdk.log"
	}
	logDir := fmt.Sprintf("%s/.abelian", wd)
	err = os.MkdirAll(logDir, 0777)
	if err != nil {
		logDir = "."
	}
	initLogRotator(fmt.Sprintf("%s/sdk.log", logDir))
	setLogLevels("off")
}
