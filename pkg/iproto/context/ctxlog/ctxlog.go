// Package ctxlog contains utilities for logging accordingly to context.Context.
package ctxlog

type Context interface {
	LogPrefix() string
}
