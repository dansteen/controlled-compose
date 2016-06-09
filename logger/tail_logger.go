// TailLogger is an implementation of logger that will tail provide a log stream
// as well as, optionally, print logs to the console
package logger

import (
	cli_logger "github.com/docker/libcompose/cli/logger"
	"github.com/docker/libcompose/logger"
)

// TailLoggerFactory Implements the logger.Factory interface for TailLogger
type TailLoggerFactory struct {
	loggerFactory logger.Factory
}

// TailLogger implements logger.Logger interface with output to a stream
type TailLogger struct {
	loggerLogger logger.Logger
	name         string
	factory      *TailLoggerFactory
}

// Create implements logger.Factory.Create.
func (c *TailLoggerFactory) Create(name string) logger.Logger {
	c.loggerFactory = cli_logger.NewColorLoggerFactory()
	colorLogger := c.loggerFactory.Create(name)
	return &TailLogger{
		loggerLogger: colorLogger,
		name:         name,
		factory:      c,
	}
}

// Out implements logger.Logger.Out.
func (c *TailLogger) Out(bytes []byte) {
	c.loggerLogger.Out(bytes)
}

// Err implements logger.Logger.Err.
func (c *TailLogger) Err(bytes []byte) {
	c.loggerLogger.Err(bytes)
}

// StreamOut will return an ioReader to StdOut back to the caller which can
// be streamed from.
func (c *TailLogger) OutLines(bytes []byte) *Line {
}

// StreamErr will return an ioReader to StdErr object back to the caller which can
// be streamed from.
func (c *TailLogger) ErrLines(bytes []byte) *Line {
}
