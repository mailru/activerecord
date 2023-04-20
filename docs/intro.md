# Вступление

Простой способ организовать модель в своём приложении:

- Скачайте и установите `argen`
- Добавьте зависимость в своём пакете `go get github.com/mailru/activerecord`
- Создайте каталог `model/repository/decl`
- Создайте файлы декларации, например: `model/repository/decl/foo.go`
- Запустите генерацию `argen --path "model/repository/" --declaration "decl" --destination "cmpl"`
- Подключайте `import "..../model/repository/cmpl/foo"`
- Используйте `foo.SelectBy...()`
- Запускайте генерацию в любой момент, когда вам необходимо

Профит!

## Пример

Подсмотреть на пример можно в [https://github.com/mailru/activerecord-cookbook](activerecord-cookbook)

## Драйверы

### octopus

Используется для подключения к базам `octopus` и `tarantool` версии 1.5

Описание `iproto` [https://github.com/Vespertinus/octopus/blob/master/doc/silverbox-protocol.txt](протокола) для работы с базой

#### tarantool1.5

https://packages.debian.org/ru/buster/tarantool-lts
