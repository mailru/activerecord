package activerecord

import (
	"bytes"
	"context"
	"fmt"
	"hash"
	"hash/crc32"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Интерфейс которому должен соответствовать билдер опций подключения к конретному инстансу
type OptionInterface interface {
	GetConnectionID() string
	InstanceMode() ServerModeType
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

// Структура описывающая конкретный шард. Каждый шард может состоять из набора мастеров и реплик
type Shard struct {
	Masters    []ShardInstance
	Replicas   []ShardInstance
	curMaster  int32
	curReplica int32
}

// Функция выбирающая следующий доступный инстанс мастера в конкретном шарде
func (s *Shard) NextMaster() ShardInstance {
	length := len(s.Masters)
	switch length {
	case 0:
		panic("no master configured")
	case 1:
		master := s.Masters[0]
		if master.Offline {
			panic("no available master")
		}

		return master
	}

	// Из-за гонок при большом кол-ве недоступных инстансов может потребоватся много попыток найти доступный узел
	attempt := 10 * length

	for i := 0; i < attempt; i++ {
		newVal := atomic.AddInt32(&s.curMaster, 1)
		newValMod := newVal % int32(len(s.Masters))

		if newValMod != newVal {
			atomic.CompareAndSwapInt32(&s.curMaster, newVal, newValMod)
		}

		master := s.Masters[newValMod]
		if master.Offline {
			continue
		}

		return master
	}

	//nolint:gosec
	// Есть небольшая вероятность при большой нагрузке и большом проценте недоступных инстансов можно залипнуть на доступном узле
	// Чтобы не паниковать выбираем рандомный узел
	return s.Masters[rand.Int()%length]
}

// Инстанс выбирающий следующий доступный инстанс реплики в конкретном шарде
func (s *Shard) NextReplica() ShardInstance {
	length := len(s.Replicas)
	if length == 1 && !s.Replicas[0].Offline {
		return s.Replicas[0]
	}

	// Из-за гонок при большом кол-ве недоступных инстансов может потребоватся много попыток найти доступный узел
	attempt := 10 * length

	for i := 0; i < attempt; i++ {
		newVal := atomic.AddInt32(&s.curReplica, 1)
		newValMod := newVal % int32(len(s.Replicas))

		if newValMod != newVal {
			atomic.CompareAndSwapInt32(&s.curReplica, newVal, newValMod)
		}

		replica := s.Replicas[newValMod]
		if replica.Offline {
			continue
		}

		return replica
	}

	//nolint:gosec
	// Есть небольшая вероятность при большой нагрузке и большом проценте недоступных инстансов поиск может залипнуть на недоступном узле
	// Чтобы не паниковать выбираем рандомный узел
	return s.Replicas[rand.Int()%length]
}

// Instances копия списка конфигураций всех инстансов шарды. В начале списка следуют мастера, потом реплики
func (c *Shard) Instances() []ShardInstance {
	instances := make([]ShardInstance, 0, len(c.Masters)+len(c.Replicas))
	instances = append(instances, c.Masters...)
	// сортировка в подсписках чтобы не зависет от порядка в котором инстансы добавлялись в конфигурацию
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].ParamsID < instances[j].ParamsID
	})

	instances = append(instances, c.Replicas...)

	replicas := instances[len(c.Masters):]
	sort.Slice(replicas, func(i, j int) bool {
		return replicas[i].ParamsID < replicas[j].ParamsID
	})

	return instances
}

// Тип описывающий кластер. Сейчас кластер - это набор шардов.
type Cluster struct {
	m      sync.RWMutex
	shards []Shard
	hash   hash.Hash
}

func NewCluster(shardCnt int) *Cluster {
	return &Cluster{
		m:      sync.RWMutex{},
		shards: make([]Shard, 0, shardCnt),
		hash:   crc32.NewIEEE(),
	}
}

// NextMaster выбирает следующий доступный инстанс мастера в шарде shardNum
func (c *Cluster) NextMaster(shardNum int) ShardInstance {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.shards[shardNum].NextMaster()
}

// NextMaster выбирает следующий доступный инстанс реплики в шарде shardNum
func (c *Cluster) NextReplica(shardNum int) (ShardInstance, bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	for _, replica := range c.shards[shardNum].Replicas {
		if replica.Offline {
			continue
		}

		return c.shards[shardNum].NextReplica(), true
	}

	return ShardInstance{}, false

}

// Append добавляет новый шард в кластер
func (c *Cluster) Append(shard Shard) {
	c.m.Lock()
	defer c.m.Unlock()

	c.shards = append(c.shards, shard)

	c.hash.Reset()
	for i := 0; i < len(c.shards); i++ {
		for _, instance := range c.shards[i].Instances() {
			c.hash.Write([]byte(instance.ParamsID))
		}
	}
}

// ShardInstances копия всех инстансов из шарды shardNum
func (c *Cluster) ShardInstances(shardNum int) []ShardInstance {
	c.m.Lock()
	defer c.m.Unlock()

	return c.shards[shardNum].Instances()
}

// Shards кол-во доступных шард кластера
func (c *Cluster) Shards() int {
	return len(c.shards)
}

// SetShardInstances заменяет инстансы кластера в шарде shardNum на инстансы из instances
func (c *Cluster) SetShardInstances(shardNum int, instances []ShardInstance) {
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

// Equal сравнивает загруженные конфигурации кластеров на основе контрольной суммы всех инстансов кластера
func (c *Cluster) Equal(c2 *Cluster) bool {
	if c == nil {
		return false
	}

	if c2 == nil {
		return false
	}

	return bytes.Equal(c.hash.Sum(nil), c2.hash.Sum(nil))
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
func NewClusterInfo(opts ...clusterOption) *Cluster {
	cl := NewCluster(0)

	for _, opt := range opts {
		opt.apply(cl)
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

	if exMaxShardOK {
		// Если используется много шардов
		for f := 0; f < shardCnt; f++ {
			shard, err := getShardInfoFromCfg(ctx, path+"/"+strconv.Itoa(f), globs, optionCreator)
			if err != nil {
				return nil, fmt.Errorf("can't get shard %d info: %w", f, err)
			}

			cluster.Append(shard)
		}
	} else {
		// Когда только один шард
		shard, err := getShardInfoFromCfg(ctx, path, globs, optionCreator)
		if err != nil {
			return nil, fmt.Errorf("can't get shard info: %w", err)
		}

		cluster.Append(shard)
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
	lock       sync.RWMutex
	container  map[string]*Cluster
	updateTime time.Time
}

// Конструктор для создания нового кешера конфигов
func NewConfigCacher() *DefaultConfigCacher {
	return &DefaultConfigCacher{
		lock:       sync.RWMutex{},
		container:  make(map[string]*Cluster),
		updateTime: time.Now(),
	}
}

// Получение конфигурации. Если есть в кеше и он еще валидный, то конфигурация берётся из кешаб
// если в кеше нет, то достаём из конфига и кешируем.
func (cc *DefaultConfigCacher) Get(ctx context.Context, path string, globs MapGlobParam, optionCreator func(ShardInstanceConfig) (OptionInterface, error)) (*Cluster, error) {
	curConf := cc.container[path]
	if cc.lock.TryLock() {
		if cc.updateTime.Sub(Config().GetLastUpdateTime()) < 0 {
			// Очищаем кеш если поменялся конфиг
			cc.container = make(map[string]*Cluster)
			cc.updateTime = time.Now()
		}
		cc.lock.Unlock()
	}

	cc.lock.RLock()
	conf, ex := cc.container[path]
	cc.lock.RUnlock()

	if !ex {
		return cc.loadClusterInfo(ctx, curConf, path, globs, optionCreator)
	}

	return conf, nil
}

func (cc *DefaultConfigCacher) loadClusterInfo(ctx context.Context, curConf *Cluster, path string, globs MapGlobParam, optionCreator func(ShardInstanceConfig) (OptionInterface, error)) (*Cluster, error) {
	cc.lock.Lock()
	defer cc.lock.Unlock()

	conf, err := GetClusterInfoFromCfg(ctx, path, globs, optionCreator)
	if err != nil {
		return nil, fmt.Errorf("can't get config: %w", err)
	}

	if conf.Equal(curConf) {
		conf = curConf
	}

	cc.container[path] = conf

	return conf, nil
}
