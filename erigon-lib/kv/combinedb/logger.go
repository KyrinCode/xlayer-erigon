package combinedb

import (
	"fmt"
	"github.com/ledgerwatch/log/v3"
)

type combineLogger struct {
	prefix string
}

func newCombinLogger(prefix string) *combineLogger {
	return &combineLogger{
		prefix: fmt.Sprintf("combindb %s", prefix),
	}
}

func (cl *combineLogger) getPrefix() string {
	return cl.prefix
}

func (cl *combineLogger) Fatalf(format string, args ...interface{}) {
	log.Error(cl.prefix, fmt.Sprintf(format, args...))
	panic("fatal error")
}

func (cl *combineLogger) Errorf(format string, args ...interface{}) {
	log.Error(cl.prefix, fmt.Sprintf(format, args...))
}

func (cl *combineLogger) Warnf(format string, args ...interface{}) {
	log.Warn(cl.prefix, fmt.Sprintf(format, args...))
}

func (cl *combineLogger) Infof(format string, args ...interface{}) {
	log.Info(cl.prefix, fmt.Sprintf(format, args...))
}

func (cl *combineLogger) Debugf(format string, args ...interface{}) {
	log.Debug(cl.prefix, fmt.Sprintf(format, args...))
}

func (cl *combineLogger) Tracef(format string, args ...interface{}) {
	log.Trace(cl.prefix, fmt.Sprintf(format, args...))
}
