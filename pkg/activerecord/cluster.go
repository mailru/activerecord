package activerecord

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
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
	Offline  bool
}

func (s *ShardInstance) IsOffline() bool {
	return s.Offline
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
	masters := Online(s.Masters)
	length := len(masters)
	switch length {
	case 0:
		panic("no master configured")
	case 1:
		return masters[0]
	}

	newVal := atomic.AddInt32(&s.curMaster, 1)
	newValMod := newVal % int32(len(masters))

	if newValMod != newVal {
		atomic.CompareAndSwapInt32(&s.curMaster, newVal, newValMod)
	}

	return masters[newValMod]
}

func Online(shards []ShardInstance) []ShardInstance {
	ret := make([]ShardInstance, 0, len(shards))
	for _, replica := range shards {
		if replica.Offline {
			continue
		}

		ret = append(ret, replica)
	}

	return ret
}

// Инстанс выбирающий конкретный инстанс реплики в конкретном шарде
func (s *Shard) NextReplica() ShardInstance {
	replicas := Online(s.Replicas)
	length := len(replicas)
	if length == 1 {
		return replicas[0]
	}

	newVal := atomic.AddInt32(&s.curReplica, 1)
	newValMod := newVal % int32(len(replicas))

	if newValMod != newVal {
		atomic.CompareAndSwapInt32(&s.curReplica, newVal, newValMod)
	}

	return replicas[newValMod]
}

// Тип описывающий кластер. Сейчас кластер - это набор шардов.
//type Cluster []Shard

type Cluster struct {
	m      sync.RWMutex
	shards []Shard
}

func NewCluster(shardCnt int) *Cluster {
	return &Cluster{
		m:      sync.RWMutex{},
		shards: make([]Shard, 0, shardCnt),
	}
}

func (c *Cluster) NextMaster(shardNum int) ShardInstance {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.shards[shardNum].NextMaster()
}

func (c *Cluster) NextReplica(shardNum int) ShardInstance {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.shards[shardNum].NextReplica()
}

func (c *Cluster) OnlineReplicas(i int) []ShardInstance {
	c.m.RLock()
	defer c.m.RUnlock()

	return Online(c.shards[i].Replicas)
}

func (c *Cluster) append(shard Shard) {
	c.m.Lock()
	defer c.m.Unlock()

	c.shards = append(c.shards, shard)
}

func (c *Cluster) Shard(i int) *Shard {
	c.m.RLock()
	defer c.m.RUnlock()

	return &c.shards[i]
}

func (c *Cluster) Len() int {
	if c == nil {
		return 0
	}

	return len(c.shards)
}

func (c *Cluster) setShardInstances(shardNum int, instances []ShardInstance) {
	c.m.Lock()
	defer c.m.Unlock()

	shard := c.shards[shardNum]
	shard.Masters = shard.Masters[:0]
	shard.Replicas = shard.Replicas[:0]
	for _, shardInstance := range instances {
		switch shardInstance.Config.Mode {
		case ModeMaster:
			shard.Masters = append(shard.Masters, shardInstance)
		case ModeReplica:
			shard.Replicas = append(shard.Replicas, shardInstance)
		}
	}

	c.shards[shardNum] = shard

}

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
func GetClusterInfoFromCfg(ctx context.Context, path string, globs MapGlobParam, optionCreator func(ShardInstanceConfig) (OptionInterface, error)) (*Cluster, error) {
	cfg := Config()

	shardCnt, exMaxShardOK := cfg.GetIntIfExists(ctx, path+"/max-shard")
	if !exMaxShardOK {
		shardCnt = 1
	}

	cluster := NewCluster(shardCnt)

	globalTimeout, exGlobalTimeout := cfg.GetDurationIfExists(ctx, path+"/Timeout")
	if exGlobalTimeout {
		globs.Timeout = globalTimeout
	}

	globalPoolSize, exGlobalPoolSize := cfg.GetIntIfExists(ctx, path+"/PoolSize")
	if !exGlobalPoolSize {
		globalPoolSize = 1
	}

	globs.PoolSize = globalPoolSize

	//var err error

	if exMaxShardOK {
		// Если используется много шардов
		for f := 0; f < shardCnt; f++ {
			shard, err := getShardInfoFromCfg(ctx, path+"/"+strconv.Itoa(f), globs, optionCreator)
			if err != nil {
				return nil, fmt.Errorf("can't get shard %d info: %w", f, err)
			}

			cluster.append(shard)
		}
	} else {
		// Когда только один шард
		shard, err := getShardInfoFromCfg(ctx, path, globs, optionCreator)
		if err != nil {
			return nil, fmt.Errorf("can't get shard info: %w", err)
		}

		cluster.append(shard)
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

	var instances []ShardInstance
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

			instances = append(instances, ShardInstance{
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

			instances = append(instances, ShardInstance{
				ParamsID: opt.GetConnectionID(),
				Config:   shardCfg,
				Options:  opt,
			})
		}
	}

	for _, shardInstance := range instances {
		switch shardInstance.Config.Mode {
		case ModeMaster:
			ret.Masters = append(ret.Masters, shardInstance)
		case ModeReplica:
			ret.Replicas = append(ret.Replicas, shardInstance)
		}
	}

	return ret, nil
}

// Структура для кеширования полученных конфигураций. Инвалидация происходит посредством
// сравнения updateTime сданной труктуры и самого конфига.
// Используется для шаринга конфигов между можелями если они используют одну и ту же
// конфигурацию для подключений
type DefaultConfigCacher struct {
	lock                 sync.RWMutex
	container            map[string]*Cluster
	updateTime           time.Time
	defaultGlobs         MapGlobParam
	defaultOptionCreator func(ShardInstanceConfig) (OptionInterface, error)
}

func NewDefaultConfigCacher(globs MapGlobParam, optionCreator func(ShardInstanceConfig) (OptionInterface, error)) *DefaultConfigCacher {
	cc := newConfigCacher()
	cc.defaultGlobs = globs
	cc.defaultOptionCreator = optionCreator

	return cc
}

// Конструктор для создания нового кешера конфигов
func newConfigCacher() *DefaultConfigCacher {
	return &DefaultConfigCacher{
		lock:       sync.RWMutex{},
		container:  make(map[string]*Cluster),
		updateTime: time.Now(),
	}
}

// Получение конфигурации. Если есть в кеше и он еще валидный, то конфигурация берётся из кешаб
// если в кеше нет, то достаём из конфига и кешируем.
func (cc *DefaultConfigCacher) Get(ctx context.Context, path string, globs MapGlobParam, optionCreator func(ShardInstanceConfig) (OptionInterface, error)) (*Cluster, error) {
	if cc.updateTime.Sub(Config().GetLastUpdateTime()) < 0 {
		// Очищаем кеш если поменялся конфиг
		cc.container = make(map[string]*Cluster)
		cc.updateTime = time.Now()
	}

	cc.lock.RLock()
	conf, ex := cc.container[path]
	cc.lock.RUnlock()

	if !ex {
		return cc.loadClusterInfo(ctx, path, globs, optionCreator)
	}

	return conf, nil
}

func (cc *DefaultConfigCacher) loadClusterInfo(ctx context.Context, path string, globs MapGlobParam, optionCreator func(ShardInstanceConfig) (OptionInterface, error)) (*Cluster, error) {
	cc.lock.Lock()
	defer cc.lock.Unlock()

	conf, err := GetClusterInfoFromCfg(ctx, path, globs, optionCreator)
	if err != nil {
		return nil, fmt.Errorf("can't get config: %w", err)
	}

	cc.container[path] = conf

	return conf, nil
}

// Актуализирует конфигурацию кластера path, синхронизируя состояние конфигурации узлов кластера с серверной стороной (тип узла и его доступность)
// Проверка типа и доступности узлов выполняется с помощью функции instanceChecker
func (cc *DefaultConfigCacher) Actualize(ctx context.Context, path string, instanceChecker func(ctx context.Context, instance ShardInstance) (ServerModeType, error)) (*Cluster, error) {
	if instanceChecker == nil {
		return nil, nil
	}

	clusterConfig, ex := cc.container[path]
	if !ex {
		var err error

		clusterConfig, err = cc.loadClusterInfo(ctx, path, cc.defaultGlobs, cc.defaultOptionCreator)
		if err != nil {
			return nil, fmt.Errorf("can't load cluster info: %w", err)
		}
	}

	for i := 0; i < clusterConfig.Len(); i++ {
		shard := clusterConfig.Shard(i)

		var instances []ShardInstance

		eg := &errgroup.Group{}

		instancesCh := make(chan ShardInstance)

		for _, si := range append(shard.Masters, shard.Replicas...) {
			si := si
			eg.Go(func() error {
				serverType, connErr := instanceChecker(ctx, si)

				if connErr == nil {
					si.Config.Mode = serverType
				}

				instancesCh <- ShardInstance{
					ParamsID: si.ParamsID,
					Config:   si.Config,
					Options:  si.Options,
					Offline:  connErr != nil,
				}

				return nil
			})
		}

		egAcc := &errgroup.Group{}

		egAcc.Go(func() error {
			for instance := range instancesCh {
				instances = append(instances, instance)
			}

			return nil
		})

		_ = eg.Wait()
		close(instancesCh)
		_ = egAcc.Wait()

		clusterConfig.setShardInstances(i, instances)
	}

	cc.lock.Lock()
	cc.container[path] = clusterConfig
	cc.lock.Unlock()

	return clusterConfig, nil
}
