package activerecord

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Интерфейс которому должен соответствовать билдер опций подключения к конретному инстансу
type OptionInterface interface {
	GetConnectionID() string
	InstanceMode() any
}

// Тип и константы для выбора инстанса в шарде
type ShardInstanceType uint8

const (
	MasterInstanceType          ShardInstanceType = iota // Любой из описанных мастеров. По умолчанию используется для rw запросов
	ReplicaInstanceType                                  // Любой из описанных реплик
	ReplicaOrMasterInstanceType                          // Любая реплика если есть, если нет то любой мастер. По умолчанию используется при селекте
)

// Тип и константы для определения режима работы конкретного инстанса.
type ServerModeType uint8

const (
	ModeMaster ServerModeType = iota
	ModeReplica
)

// Структура используется для описания конфигурации конктретного инстанса
type ShardInstanceConfig struct {
	Timeout  time.Duration
	Mode     ServerModeType
	PoolSize int
	Addr     string
}

// Структура описывающая инстанс в кластере
type ShardInstance struct {
	ParamsID string
	Config   ShardInstanceConfig
	Options  interface{}
}

// Структура описывающая конкретный шард. Каждый шард может состоять из набора мастеров и реплик
type Shard struct {
	Masters    []ShardInstance
	Replicas   []ShardInstance
	curMaster  int32
	curReplica int32
}

// Функция выбирающая следующий инстанс мастера в конкретном шарде
func (s *Shard) NextMaster() ShardInstance {
	length := len(s.Masters)
	switch length {
	case 0:
		panic("no master configured")
	case 1:
		return s.Masters[0]
	}

	newVal := atomic.AddInt32(&s.curMaster, 1)
	newValMod := newVal % int32(len(s.Masters))

	if newValMod != newVal {
		atomic.CompareAndSwapInt32(&s.curMaster, newVal, newValMod)
	}

	return s.Masters[newValMod]
}

// Инстанс выбирающий конкретный инстанс реплики в конкретном шарде
func (s *Shard) NextReplica() ShardInstance {
	length := len(s.Replicas)
	if length == 1 {
		return s.Replicas[0]
	}

	newVal := atomic.AddInt32(&s.curReplica, 1)
	newValMod := newVal % int32(len(s.Replicas))

	if newValMod != newVal {
		atomic.CompareAndSwapInt32(&s.curReplica, newVal, newValMod)
	}

	return s.Replicas[newValMod]
}

// Тип описывающий кластер. Сейчас кластер - это набор шардов.
type Cluster []Shard

// Тип используемый для передачи набора значений по умолчанию для параметров
type MapGlobParam struct {
	Timeout  time.Duration
	PoolSize int
}

// Конструктор который позволяет проинициализировать новый кластер. В опциях передаются все шарды,
// сколько шардов, столько и опций. Используется в случаях, когда информация по кластеру прописана
// непосредственно в декларации модели, а не в конфиге.
// Так же используется при тестировании.
func NewClusterInfo(opts ...clusterOption) Cluster {
	cl := Cluster{}

	for _, opt := range opts {
		opt.apply(&cl)
	}

	return cl
}

// Констркуктор позволяющий проинициализировать кластер их конфигурации.
// На вход передаётся путь в конфиге, значения по умолчанию, и ссылка на функцию, которая
// создаёт структуру опций и считает контрольную сумму, для того, что бы следить за их изменением в онлайне.
func GetClusterInfoFromCfg(ctx context.Context, path string, globs MapGlobParam, optionCreator func(ShardInstanceConfig) (OptionInterface, error)) (Cluster, error) {
	cfg := Config()

	shardCnt, exMaxShardOK := cfg.GetIntIfExists(ctx, path+"/max-shard")
	if !exMaxShardOK {
		shardCnt = 1
	}

	cluster := make(Cluster, shardCnt)

	globalTimeout, exGlobalTimeout := cfg.GetDurationIfExists(ctx, path+"/Timeout")
	if exGlobalTimeout {
		globs.Timeout = globalTimeout
	}

	globalPoolSize, exGlobalPoolSize := cfg.GetIntIfExists(ctx, path+"/PoolSize")
	if !exGlobalPoolSize {
		globalPoolSize = 1
	}

	globs.PoolSize = globalPoolSize

	var err error

	if exMaxShardOK {
		// Если используется много шардов
		for f := 0; f < shardCnt; f++ {
			cluster[f], err = getShardInfoFromCfg(ctx, path+"/"+strconv.Itoa(f), globs, optionCreator)
			if err != nil {
				return nil, fmt.Errorf("can't get shard %d info: %w", f, err)
			}
		}
	} else {
		// Когда только один шард
		cluster[0], err = getShardInfoFromCfg(ctx, path, globs, optionCreator)
		if err != nil {
			return nil, fmt.Errorf("can't get shard info: %w", err)
		}
	}

	return cluster, nil
}

// Чтение информации по конкретному шарду из конфига
func getShardInfoFromCfg(ctx context.Context, path string, globParam MapGlobParam, optionCreator func(ShardInstanceConfig) (OptionInterface, error)) (Shard, error) {
	cfg := Config()
	ret := Shard{
		Masters:  []ShardInstance{},
		Replicas: []ShardInstance{},
	}

	shardTimeout := cfg.GetDuration(ctx, path+"/Timeout", globParam.Timeout)
	shardPoolSize := cfg.GetInt(ctx, path+"/PoolSize", globParam.PoolSize)

	// информация по местерам
	master, exMaster := cfg.GetStringIfExists(ctx, path+"/master")
	if !exMaster {
		master, exMaster = cfg.GetStringIfExists(ctx, path)
		if !exMaster {
			return Shard{}, fmt.Errorf("master should be specified in '%s' or in '%s/master' and replica in '%s/replica'", path, path, path)
		}
	}

	if master != "" {
		for _, inst := range strings.Split(master, ",") {
			if inst == "" {
				return Shard{}, fmt.Errorf("invalid master instance options: addr is empty")
			}

			shardCfg := ShardInstanceConfig{
				Addr:     inst,
				Mode:     ModeMaster,
				PoolSize: shardPoolSize,
				Timeout:  shardTimeout,
			}

			opt, err := optionCreator(shardCfg)
			if err != nil {
				return Shard{}, fmt.Errorf("can't create instanceOption: %w", err)
			}

			ret.Masters = append(ret.Masters, ShardInstance{
				ParamsID: opt.GetConnectionID(),
				Config:   shardCfg,
				Options:  opt,
			})
		}
	}

	// Информация по репликам
	replica, exReplica := cfg.GetStringIfExists(ctx, path+"/replica")
	if exReplica {
		for _, inst := range strings.Split(replica, ",") {
			if inst == "" {
				return Shard{}, fmt.Errorf("invalid slave instance options: addr is empty")
			}

			shardCfg := ShardInstanceConfig{
				Addr:     inst,
				Mode:     ModeReplica,
				PoolSize: shardPoolSize,
				Timeout:  shardTimeout,
			}

			opt, err := optionCreator(shardCfg)
			if err != nil {
				return Shard{}, fmt.Errorf("can't create instanceOption: %w", err)
			}

			ret.Replicas = append(ret.Replicas, ShardInstance{
				ParamsID: opt.GetConnectionID(),
				Config:   shardCfg,
				Options:  opt,
			})
		}
	}

	return ret, nil
}

// Структура для кеширования полученных конфигураций. Инвалидация происходит посредством
// сравнения updateTime сданной труктуры и самого конфига.
// Используется для шаринга конфигов между можелями если они используют одну и ту же
// конфигурацию для подключений
type DefaultConfigCacher struct {
	lock       sync.Mutex
	container  map[string]Cluster
	updateTime time.Time
}

// Конструктор для создания нового кешера конфигов
func newConfigCacher() *DefaultConfigCacher {
	return &DefaultConfigCacher{
		lock:       sync.Mutex{},
		container:  make(map[string]Cluster),
		updateTime: time.Now(),
	}
}

func (cc *DefaultConfigCacher) GetNoOps(ctx context.Context, path string) (Cluster, bool) {
	cc.lock.Lock()
	defer cc.lock.Unlock()

	conf, ex := cc.container[path]

	return conf, ex
}

// Получение конфигурации. Если есть в кеше и он еще валидный, то конфигурация берётся из кешаб
// если в кеше нет, то достаём из конфига и кешируем.
func (cc *DefaultConfigCacher) Get(ctx context.Context, path string, globs MapGlobParam, optionCreator func(ShardInstanceConfig) (OptionInterface, error)) (Cluster, error) {
	cc.lock.Lock()
	defer cc.lock.Unlock()

	if cc.updateTime.Sub(Config().GetLastUpdateTime()) < 0 {
		// Очищаем кеш если поменялся конфиг
		cc.container = make(map[string]Cluster)
		cc.updateTime = time.Now()
	}

	conf, ex := cc.container[path]

	if !ex {
		var err error

		conf, err = GetClusterInfoFromCfg(ctx, path, globs, optionCreator)
		if err != nil {
			return Cluster{}, fmt.Errorf("can't get config: %w", err)
		}

		cc.container[path] = conf
	}

	return conf, nil
}
