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
		prefix: prefix,
	}
}

func (cl *combineLogger) getPrefix() string {
	return cl.prefix
}

func (cl *combineLogger) Fatal(msg string, args ...interface{}) {
	args = append([]interface{}{"msg", msg}, args...)
	log.Error(cl.prefix, args...)
	panic("fatal error")
}

func (cl *combineLogger) Fatalf(format string, args ...interface{}) {
	cl.Fatal(fmt.Sprintf(format, args...))
}

func (cl *combineLogger) Error(msg string, args ...interface{}) {
	args = append([]interface{}{"msg", msg}, args...)
	log.Error(cl.prefix, args)
}

func (cl *combineLogger) Errorf(format string, args ...interface{}) {
	cl.Error(fmt.Sprintf(format, args...))
}

func (cl *combineLogger) Warn(msg string, args ...interface{}) {
	args = append([]interface{}{"msg", msg}, args...)
	log.Warn(cl.prefix, args)
}

func (cl *combineLogger) Warnf(format string, args ...interface{}) {
	cl.Warn(fmt.Sprintf(format, args...))
}

func (cl *combineLogger) Info(msg string, args ...interface{}) {
	args = append([]interface{}{"msg", msg}, args...)
	log.Info(cl.prefix, args)
}

func (cl *combineLogger) Infof(format string, args ...interface{}) {
	log.Info(fmt.Sprintf(format, args...))
}

func (cl *combineLogger) Debug(msg string, args ...interface{}) {
	args = append([]interface{}{"msg", msg}, args...)
	log.Debug(cl.prefix, args)
}

func (cl *combineLogger) Debugf(format string, args ...interface{}) {
	cl.Debug(fmt.Sprintf(format, args...))
}

func (cl *combineLogger) Trace(msg string, args ...interface{}) {
	args = append([]interface{}{"msg", msg}, args...)
	log.Trace(cl.prefix, args)
}

func (cl *combineLogger) Tracef(msg string, args ...interface{}) {
	cl.Trace(fmt.Sprintf(msg, args...))
}
