package activerecord

// Структура используемая для сбора статистики по запросам
type MockerLogger struct {
	// Название моккера
	MockerName string

	// Вызовы моккера для создания списка моков
	Mockers string

	// Получение списка фикстур для моков
	FixturesSelector string

	// Название пакета для которого необходимо добавить моки
	ResultName string

	// Результат возвращаемый селектором
	Results any
}
