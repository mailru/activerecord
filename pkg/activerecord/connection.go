package activerecord

import (
	"context"
	"fmt"
	"sync"
)

type ConnectionInterface interface {
	Close()
	Done() <-chan struct{}
}

type connectionPool struct {
	lock      sync.Mutex
	container map[string]ConnectionInterface
}

func newConnectionPool() *connectionPool {
	return &connectionPool{
		lock:      sync.Mutex{},
		container: make(map[string]ConnectionInterface),
	}
}

// TODO при долгом неиспользовании какого то пула надо закрывать его. Это для случаев когда в конфиге поменялась конфигурация
// надо зачищать старые пулы, что бы освободить конекты.
// если будут колбеки о том, что сменилась конфигурация то можно подчищать по этим колбекам.
func (cp *connectionPool) add(shard ShardInstance, connector func(interface{}) (ConnectionInterface, error)) (ConnectionInterface, error) {
	if _, ex := cp.container[shard.ParamsID]; ex {
		return nil, fmt.Errorf("attempt to add duplicate connID: %s", shard.ParamsID)
	}

	pool, err := connector(shard.Options)
	if err != nil {
		return nil, fmt.Errorf("error add connection to shard: %w", err)
	}

	cp.container[shard.ParamsID] = pool

	return pool, nil
}

func (cp *connectionPool) Add(shard ShardInstance, connector func(interface{}) (ConnectionInterface, error)) (ConnectionInterface, error) {
	cp.lock.Lock()
	defer cp.lock.Unlock()

	return cp.add(shard, connector)
}

func (cp *connectionPool) GetOrAdd(shard ShardInstance, connector func(interface{}) (ConnectionInterface, error)) (ConnectionInterface, error) {
	cp.lock.Lock()
	defer cp.lock.Unlock()

	var err error

	conn := cp.Get(shard)
	if conn == nil {
		conn, err = cp.add(shard, connector)
	}

	return conn, err
}

func (cp *connectionPool) Get(shard ShardInstance) ConnectionInterface {
	if conn, ex := cp.container[shard.ParamsID]; ex {
		return conn
	}

	return nil
}

func (cp *connectionPool) CloseConnection(ctx context.Context) {
	cp.lock.Lock()

	for name, pool := range cp.container {
		pool.Close()
		Logger().Debug(ctx, "connection close: %s", name)
	}

	for _, pool := range cp.container {
		<-pool.Done()
		Logger().Debug(ctx, "pool closed done")
	}

	cp.lock.Unlock()
}
