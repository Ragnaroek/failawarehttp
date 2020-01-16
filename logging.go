package http

type Logger interface {
	Debugf(format string, v ...interface{})
}
