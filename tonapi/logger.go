package tonapi

type Logger interface {
	Errorf(format string, args ...interface{})
}

type noopLogger struct{}

func (l noopLogger) Errorf(format string, args ...interface{}) {}
