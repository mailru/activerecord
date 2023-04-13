package activerecord

import (
	"context"
	"fmt"
	"time"
)

type DefaultConfig struct {
	cfg     map[string]interface{}
	created time.Time
}

func NewDefaultConfig() *DefaultConfig {
	return &DefaultConfig{
		cfg: make(map[string]interface{}),
	}
}

func NewDefaultConfigFromMap(cfg map[string]interface{}) *DefaultConfig {
	return &DefaultConfig{
		cfg:     cfg,
		created: time.Now(),
	}
}

func (dc *DefaultConfig) GetLastUpdateTime() time.Time {
	return dc.created
}

func (dc *DefaultConfig) GetBool(ctx context.Context, confPath string, dfl ...bool) bool {
	if ret, ok := dc.GetBoolIfExists(ctx, confPath); ok {
		return ret
	}

	if len(dfl) != 0 {
		return dfl[0]
	}

	return false
}

func (dc *DefaultConfig) GetBoolIfExists(ctx context.Context, confPath string) (value bool, ok bool) {
	if param, ex := dc.cfg[confPath]; ex {
		if ret, ok := param.(bool); ok {
			return ret, true
		}

		Logger().Warn(ctx, fmt.Sprintf("param %s has type %T, want bool", confPath, param))
	}

	return false, false
}

func (dc *DefaultConfig) GetInt(ctx context.Context, confPath string, dfl ...int) int {
	if ret, ok := dc.GetIntIfExists(ctx, confPath); ok {
		return ret
	}

	if len(dfl) != 0 {
		return dfl[0]
	}

	return 0
}

func (dc *DefaultConfig) GetIntIfExists(ctx context.Context, confPath string) (int, bool) {
	if param, ex := dc.cfg[confPath]; ex {
		if ret, ok := param.(int); ok {
			return ret, true
		}

		Logger().Warn(ctx, fmt.Sprintf("param %s has type %T, want int", confPath, param))
	}

	return 0, false
}

func (dc *DefaultConfig) GetDuration(ctx context.Context, confPath string, dfl ...time.Duration) time.Duration {
	if ret, ok := dc.GetDurationIfExists(ctx, confPath); ok {
		return ret
	}

	if len(dfl) != 0 {
		return dfl[0]
	}

	return 0
}

func (dc *DefaultConfig) GetDurationIfExists(ctx context.Context, confPath string) (time.Duration, bool) {
	if param, ex := dc.cfg[confPath]; ex {
		if ret, ok := param.(time.Duration); ok {
			return ret, true
		}

		Logger().Warn(ctx, fmt.Sprintf("param %s has type %T, want time.Duration", confPath, param))
	}

	return 0, false
}

func (dc *DefaultConfig) GetString(ctx context.Context, confPath string, dfl ...string) string {
	if ret, ok := dc.GetStringIfExists(ctx, confPath); ok {
		return ret
	}

	if len(dfl) != 0 {
		return dfl[0]
	}

	return ""
}

func (dc *DefaultConfig) GetStringIfExists(ctx context.Context, confPath string) (string, bool) {
	if param, ex := dc.cfg[confPath]; ex {
		if ret, ok := param.(string); ok {
			return ret, true
		}

		Logger().Warn(ctx, fmt.Sprintf("param %s has type %T, want string", confPath, param))
	}

	return "", false
}

func (dc *DefaultConfig) GetStrings(ctx context.Context, confPath string, dfl []string) []string {
	return []string{}
}

func (dc *DefaultConfig) GetStruct(ctx context.Context, confPath string, valuePtr interface{}) (bool, error) {
	return false, nil
}
