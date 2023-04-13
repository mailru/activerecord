# Рецепты

В этом документе представлены основные рецепты по использованию библиотеки.
Эта библиотека представляет из себя набор пакетов для подключения в приложение и утилиту для генерации.

## Декларативное описание

### Декларирование конфигурации хранилища

```go
//ar:shard_by_func:shard_func
//ar:shard_by_field:Id:7
//ar:serverHost:127.0.0.1;serverPort:12345;serverTimeout:500;serverUser:test;serverPass:test
//ar:backend:octopus,tarantool
```

### Декларирование полей

### Декларирование связанных сущностей

### Декларирование индексов

#### Частичные индексы

### Декларирование триггеров

### Декларирование флагов

## Конфигурирование

### Интерфейс конфига

Интерфейс конфига очень схож с реализацией `onlineconf`.

Но на самом деле можно реализовать любую структуру, которая ему удовлетворяет.

Например, внутри проекта в котором вы используете AR, можно создать структуру `ARConfig`:

```golang
type ARConfig struct {
    updatedIn time.Time
}

func NewARConfig() *ARConfig {
    arcfg := &ARConfig{
        updatedIn: time.Now(),
    }

    return arcfg
}

func (dc *ARConfig) GetLastUpdateTime() time.Time {
    return dc.updatedIn
}

func (dc *ARConfig) GetBool(ctx context.Context, confPath string, dfl ...bool) bool {
    if len(dfl) != 0 {
        return dfl[0]
    }

    return false
}
func (dc *ARConfig) GetBoolIfExists(ctx context.Context, confPath string) (value bool, ok bool) {
    return false, false
}

func (dc *ARConfig) GetDurationIfExists(ctx context.Context, confPath string) (time.Duration, bool) {
    switch confPath {
    case "arcfg/Timeout":
        return time.Millisecond * 200, true
    default:
        return 0, false
    }
}
func (dc *ARConfig) GetDuration(ctx context.Context, confPath string, dfl ...time.Duration) time.Duration {
    ret, ok := dc.GetDurationIfExists(ctx, confPath)
    if !ok && len(dfl) != 0 {
        ret = dfl[0]
    }

    return ret
}

func (dc *ARConfig) GetIntIfExists(ctx context.Context, confPath string) (int, bool) {
    switch confPath {
    case "arcfg/PoolSize":
        return 10, true
    default:
        return 0, false
    }
}
func (dc *ARConfig) GetInt(ctx context.Context, confPath string, dfl ...int) int {
    ret, ok := dc.GetIntIfExists(ctx, confPath)
    if !ok && len(dfl) != 0 {
        ret = dfl[0]
    }

    return ret
}

func (dc *ARConfig) GetStringIfExists(ctx context.Context, confPath string) (string, bool) {
    switch confPath {
    case "arcfg/master":
        return "127.0.0.1:11011", true
    case "arcfg/replica":
        return "127.0.0.1:11011", true
    default:
        return "", false
    }
}
func (dc *ARConfig) GetString(ctx context.Context, confPath string, dfl ...string) string {
    ret, ok := dc.GetStringIfExists(ctx, confPath)
    if !ok && len(dfl) != 0 {
        ret = dfl[0]
    }

    return ret
}

func (dc *ARConfig) GetStrings(ctx context.Context, confPath string, dfl []string) []string {
    return []string{}
}
func (dc *ARConfig) GetStruct(ctx context.Context, confPath string, valuePtr interface{}) (bool, error) {
    return false, nil
}
```

Это статический конфиг, и в таком виде он кажется избыточным, но можно передать при инициализации такого пакета конфиг приложения, который может
изменять свои параметры в течении времени. Тогда необходимо в методе `GetLastUpdateTime` отдавать время последнего обновления конфига.
Это позволит перечитывать параметры подключения на лету и пере-подключаться к базе.

## Атомарность на уровне БД

### Мутаторы

## Архитектурное построение

## Best practices
