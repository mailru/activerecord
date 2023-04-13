package activerecord

import (
	"context"
	"fmt"
	"log"

	"github.com/mailru/activerecord/pkg/iproto/iproto"
)

type ctxKey uint8
type ValueLogPrefix map[string]interface{}
type DefaultLogger struct {
	level  uint32
	Fields ValueLogPrefix
}

const (
	PanicLoggerLevel uint32 = iota
	FatalLoggerLevel
	ErrorLoggerLevel
	WarnLoggerLevel
	InfoLoggerLevel
	DebugLoggerLevel
	TraceLoggerLevel
)

const (
	ContextLogprefix ctxKey = iota
)

const (
	ValueContextErrorField = "context"
)

func NewLogger() *DefaultLogger {
	return &DefaultLogger{
		level:  InfoLoggerLevel,
		Fields: ValueLogPrefix{"orm": "activerecord"},
	}
}

func (l *DefaultLogger) getLoggerFromContext(ctx context.Context) LoggerInterface {
	return l.getLoggerFromContextAndValue(ctx, ValueLogPrefix{})
}

func (l *DefaultLogger) SetLoggerValueToContext(ctx context.Context, val ValueLogPrefix) context.Context {
	ctxVal := ctx.Value(ContextLogprefix)
	if ctxVal != nil {
		lprefix, ok := ctxVal.(ValueLogPrefix)
		if !ok {
			val["logger.context.error"] = ValueContextErrorField
			val["logger.context.valueType"] = fmt.Sprintf("%T", ctxVal)
		} else {
			for k, v := range lprefix {
				if _, ok := val[k]; !ok {
					val[k] = v
				}
			}
		}
	}

	return context.WithValue(ctx, ContextLogprefix, val)
}

func (l *DefaultLogger) getLoggerFromContextAndValue(ctx context.Context, addVal ValueLogPrefix) LoggerInterface {
	// Думаю что надо закешировать один раз инстанс логгера для контекста
	// Но надо учитывать, что мог измениться уровень логирования хотим ли мы в рамках одного запроса
	// менять уровни логирования?
	// Еще надо добавить в логгер конфигурацию, что бы уровни ролирования можно было
	// настраивать на уровне моделей
	nl := NewLogger()
	nl.level = l.level

	for k, v := range l.Fields {
		nl.Fields[k] = v
	}

	for k, v := range addVal {
		nl.Fields[k] = v
	}

	ctxVal := ctx.Value(ContextLogprefix)
	if ctxVal == nil {
		nl.Fields["logger.context"] = "empty"
	} else {
		lprefix, ok := ctxVal.(ValueLogPrefix)
		if !ok {
			nl.Fields["logger.context.error"] = ValueContextErrorField
			nl.Fields["logger.context.valueType"] = fmt.Sprintf("%T", ctxVal)
		} else {
			for k, v := range lprefix {
				nl.Fields[k] = v
			}
		}
	}

	return nl
}

func (l *DefaultLogger) SetLogLevel(level uint32) {
	l.level = level
}

func (l *DefaultLogger) loggerPrint(level uint32, lprefix string, args ...interface{}) {
	if l.level < level {
		return
	}

	log.Print(lprefix, l.Fields, args)
}

func (l *DefaultLogger) Debug(ctx context.Context, args ...interface{}) {
	l.getLoggerFromContext(ctx).(*DefaultLogger).loggerPrint(DebugLoggerLevel, "DEBUG: ", args)
}

func (l *DefaultLogger) Trace(ctx context.Context, args ...interface{}) {
	l.getLoggerFromContext(ctx).(*DefaultLogger).loggerPrint(TraceLoggerLevel, "TRACE: ", args)
}

func (l *DefaultLogger) Info(ctx context.Context, args ...interface{}) {
	l.getLoggerFromContext(ctx).(*DefaultLogger).loggerPrint(InfoLoggerLevel, "INFO: ", args)
}

func (l *DefaultLogger) Error(ctx context.Context, args ...interface{}) {
	l.getLoggerFromContext(ctx).(*DefaultLogger).loggerPrint(ErrorLoggerLevel, "ERROR: ", args)
}

func (l *DefaultLogger) Warn(ctx context.Context, args ...interface{}) {
	l.getLoggerFromContext(ctx).(*DefaultLogger).loggerPrint(WarnLoggerLevel, "WARN: ", args)
}

func (l *DefaultLogger) Fatal(ctx context.Context, args ...interface{}) {
	log.Fatal("FATAL: ", l.Fields, args)
}

func (l *DefaultLogger) Panic(ctx context.Context, args ...interface{}) {
	log.Panic("PANIC; ", l.Fields, args)
}

func (l *DefaultLogger) CollectQueries(ctx context.Context, f func() (MockerLogger, error)) {
}

type IprotoLogger struct{}

var _ iproto.Logger = IprotoLogger{}

func (IprotoLogger) Printf(ctx context.Context, fmtStr string, v ...interface{}) {
	ctx = Logger().SetLoggerValueToContext(ctx, map[string]interface{}{"iproto": "client"})
	Logger().Info(ctx, fmt.Sprintf(fmtStr, v...))
}

func (IprotoLogger) Debugf(ctx context.Context, fmtStr string, v ...interface{}) {
	ctx = Logger().SetLoggerValueToContext(ctx, map[string]interface{}{"iproto": "client"})
	Logger().Debug(ctx, fmt.Sprintf(fmtStr, v...))
}
