package activerecord

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
)

var ErrNoData = errors.New("no data")

type SelectorLimiter interface {
	Limit() uint32
	Offset() uint32
	FullfillWarn() bool
	fmt.Stringer
}

type Limiter struct {
	limit, offset uint32
	fullfillWarn  bool
}

func EmptyLimiter() Limiter {
	return Limiter{}
}

func NewLimiter(limit uint32) Limiter {
	return Limiter{limit: limit}
}

func NewLimitOffset(limit uint32, offset uint32) Limiter {
	return Limiter{limit: limit, offset: offset}
}

func NewThreshold(limit uint32) Limiter {
	return Limiter{limit: limit, fullfillWarn: true}
}

func (l Limiter) Offset() uint32 {
	return l.offset
}

func (l Limiter) Limit() uint32 {
	return l.limit
}

func (l Limiter) FullfillWarn() bool {
	return l.fullfillWarn
}

func (l Limiter) String() string {
	return fmt.Sprintf("Limit: %d, Offset: %d, Is Threshold: %t", l.limit, l.offset, l.fullfillWarn)
}

//go:generate mockery --name ConfigInterface --filename mock_config.go --structname MockConfig --with-expecter=true  --inpackage
type ConfigInterface interface {
	GetBool(ctx context.Context, confPath string, dfl ...bool) bool
	GetBoolIfExists(ctx context.Context, confPath string) (value bool, ok bool)
	GetInt(ctx context.Context, confPath string, dfl ...int) int
	GetIntIfExists(ctx context.Context, confPath string) (int, bool)
	GetDuration(ctx context.Context, confPath string, dfl ...time.Duration) time.Duration
	GetDurationIfExists(ctx context.Context, confPath string) (time.Duration, bool)
	GetString(ctx context.Context, confPath string, dfl ...string) string
	GetStringIfExists(ctx context.Context, confPath string) (string, bool)
	GetStrings(ctx context.Context, confPath string, dfl []string) []string
	GetStruct(ctx context.Context, confPath string, valuePtr interface{}) (bool, error)
	GetLastUpdateTime() time.Time
}

type LoggerInterface interface {
	SetLoggerValueToContext(ctx context.Context, addVal ValueLogPrefix) context.Context

	SetLogLevel(level uint32)
	Fatal(ctx context.Context, args ...interface{})
	Error(ctx context.Context, args ...interface{})
	Warn(ctx context.Context, args ...interface{})
	Info(ctx context.Context, args ...interface{})
	Debug(ctx context.Context, args ...interface{})
	Trace(ctx context.Context, args ...interface{})

	CollectQueries(ctx context.Context, f func() (MockerLogger, error))
}

type ConnectionCacherInterface interface {
	Add(shard ShardInstance, connector func(interface{}) (ConnectionInterface, error)) (ConnectionInterface, error)
	GetOrAdd(shard ShardInstance, connector func(interface{}) (ConnectionInterface, error)) (ConnectionInterface, error)
	Get(shard ShardInstance) ConnectionInterface
	CloseConnection(context.Context)
}

type PingerInterface interface {
	AddClusterChecker(ctx context.Context, path string, params ClusterConfigParameters) (*Cluster, error)
}

type ConfigCacherInterface interface {
	Get(ctx context.Context, path string, glob MapGlobParam, optionCreator func(ShardInstanceConfig) (OptionInterface, error)) (*Cluster, error)
	Actualize(ctx context.Context, path string, params ClusterConfigParameters) (*Cluster, error)
}

type SerializerInterface interface {
	Unmarshal(val interface{}) (interface{}, error)
	Marshal(data interface{}) (interface{}, error)
}

type MetricTimerInterface interface {
	Timing(ctx context.Context, name string)
	Finish(ctx context.Context, name string)
}

type MetricStatCountInterface interface {
	Inc(ctx context.Context, name string, val float64)
}

type MetricErrorCountInterface interface {
	Inc(ctx context.Context, name string, val float64)
}

type MetricInterface interface {
	StatCount(storage, entity string) MetricStatCountInterface
	ErrorCount(storage, entity string) MetricStatCountInterface
	Timer(storage, entity string) MetricTimerInterface
}

type ActiveRecord struct {
	instanceCreator  string
	config           ConfigInterface
	logger           LoggerInterface
	metric           MetricInterface
	connectionCacher ConnectionCacherInterface
	configCacher     ConfigCacherInterface
	pinger           PingerInterface
}

var instance *ActiveRecord
var createMutex sync.Mutex

func ReinitActiveRecord(opts ...Option) {
	instance = nil

	InitActiveRecord(opts...)
}

func InitActiveRecord(opts ...Option) {
	createMutex.Lock()
	defer createMutex.Unlock()

	if instance != nil {
		panic(fmt.Sprintf("can't initialise twice, first from `%s`", instance.instanceCreator))
	}

	caller := "unknown_caller"

	_, file, no, ok := runtime.Caller(1)
	if ok {
		caller = fmt.Sprintf("%s:%d ", file, no)
	}

	instance = &ActiveRecord{
		instanceCreator:  caller,
		logger:           NewLogger(),
		config:           NewDefaultConfig(),
		metric:           NewDefaultNoopMetric(),
		connectionCacher: newConnectionPool(),
		configCacher:     newConfigCacher(),
	}

	for _, opt := range opts {
		opt.apply(instance)
	}
}

func GetInstance() *ActiveRecord {
	if instance == nil {
		panic("get instance before initialization")
	}

	return instance
}

func Logger() LoggerInterface {
	return GetInstance().logger
}

func Metric() MetricInterface {
	return GetInstance().metric
}

func Config() ConfigInterface {
	return GetInstance().config
}

func ConnectionCacher() ConnectionCacherInterface {
	return GetInstance().connectionCacher
}

func ConfigCacher() ConfigCacherInterface {
	return GetInstance().configCacher
}

func AddClusterChecker(ctx context.Context, path string, params ClusterConfigParameters) (*Cluster, error) {
	if instance == nil || instance.pinger == nil || !params.Validate() {
		return nil, nil
	}

	return instance.pinger.AddClusterChecker(ctx, path, params)
}
