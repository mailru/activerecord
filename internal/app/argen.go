// Пакет app - основной пакет приложения.
//
// Приложение проходит несколько этапов парсинг, проверку и генерацию
// При инициализации указывается путь где находится описание
// и путь где сохраняются результирующие структуры.
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/checker"
	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/internal/pkg/generator"
	"github.com/mailru/activerecord/internal/pkg/parser"
)

// Структура приложения
// src и dst - исходная и конечная папки используетмые для генерации репозиториеа
// dstFixture - конечная папка для генерации тестовых фикстур для сгенерированных репозиториев
// srcEntry, dstEntry - используется для хранения содержимого
// соответствующих папок на момент запуска приложения
// packagesParsed, packagesLock - мапка с обработанными пакетами
// и мьютекс для блокировки при их добавлении
// packagesLinked - используется для хранения информации о ссылках
// appInfo - включает информацию о самом пакете, эта информация используется
// при генерации конечных фалов.
// modName - имя модуля используемое для построения путей import-а
type ArGen struct {
	ctx                context.Context
	src, dst           string
	srcEntry, dstEntry []fs.DirEntry
	packagesParsed     map[string]*ds.RecordPackage
	packagesLock       sync.Mutex
	packagesLinked     map[string]string
	appInfo            *ds.AppInfo
	modName            string
	fileToRemove       map[string]bool
	dstFixture         string
	pkgFixture         string
}

// Пропускает этап генерации сторов фикстур,
// если не задан путь к папке
func (a *ArGen) skipGenerateFixture() bool {
	return len(a.dstFixture) == 0
}

// Инициализация приложения
// информацию по параметрам см. в описании структуры ArGen
func Init(ctx context.Context, appInfo *ds.AppInfo, srcDir, dstDir, fixtureDir, modName string) (*ArGen, error) {
	argen := ArGen{
		ctx:            ctx,
		src:            srcDir,
		dst:            dstDir,
		dstFixture:     fixtureDir,
		pkgFixture:     "proc_fixture",
		srcEntry:       []fs.DirEntry{},
		dstEntry:       []fs.DirEntry{},
		packagesParsed: map[string]*ds.RecordPackage{},
		packagesLock:   sync.Mutex{},
		packagesLinked: map[string]string{},
		appInfo:        appInfo,
		modName:        modName,
		fileToRemove:   map[string]bool{},
	}

	// Подготавливаем информацию из src и dst директорий
	err := argen.prepareDir()
	if err != nil {
		return nil, fmt.Errorf("error prepare dirs: %w", err)
	}

	return &argen, nil
}

// Регулярное выражения для проверки названий пакетов
// сейчас поддерживаются пакеты состоящие исключительно из
// маленьких латинских символов, длинной не более 20
var rxPkgName = regexp.MustCompile(`^[a-z]{1,20}$`)

// Функция для добаление записи об очередном обработанном файле
func (a *ArGen) addRecordPackage(pkgName string) (*ds.RecordPackage, error) {
	a.packagesLock.Lock()
	defer a.packagesLock.Unlock()

	// Хорошее имя пакета должно быть коротким и чистым
	// Состоит только из букв в нижнем регистре, без подчеркиваний
	// Обычно это простые существительные в единственном числе
	if !rxPkgName.MatchString(pkgName) {
		return nil, &arerror.ErrParseGenDecl{Name: pkgName, Err: arerror.ErrBadPkgName}
	}

	// проверка на то, что такого пакета ранее не было
	if _, ex := a.packagesParsed[pkgName]; ex {
		return nil, &arerror.ErrParseGenDecl{Name: pkgName, Err: arerror.ErrRedefined}
	}

	a.packagesParsed[pkgName] = ds.NewRecordPackage()

	return a.packagesParsed[pkgName], nil
}

// Подготовка к проверке всех обработанных пакетов-деклараций
// Обогащаем информацию по пакетам путями для импорта и
// строим обратный индекс от имен структур к именам файлов
func (a *ArGen) prepareCheck() error {
	for key, pkg := range a.packagesParsed {
		pkg.Namespace.ModuleName = filepath.Join(a.modName, a.dst, pkg.Namespace.PackageName)
		a.packagesLinked[pkg.Namespace.PackageName] = key
		a.packagesParsed[key] = pkg
	}

	// Сбор по подготовка информации по существующим файлам в результирующей директории
	exists, err := a.getExists()
	if err != nil {
		return fmt.Errorf("can't get exists files: %s", err)
	}

	// Строим мапку по существующим файлам для определения "лишних" фалов в
	// результирующей директории
	for _, file := range exists {
		a.fileToRemove[file] = true
	}

	return nil
}

func (a *ArGen) prepareGenerate(cl *ds.RecordPackage) (map[string]ds.RecordPackage, error) {
	linkObjects := map[string]ds.RecordPackage{}
	for _, fo := range cl.FieldsObjectMap {
		linkObjects[fo.ObjectName] = *a.packagesParsed[a.packagesLinked[fo.ObjectName]]

		_, err := cl.FindOrAddImport(linkObjects[fo.ObjectName].Namespace.ModuleName, linkObjects[fo.ObjectName].Namespace.PackageName)
		if err != nil {
			return nil, fmt.Errorf("error process `%s` linkObject for package `%s`: %s", fo.ObjectName, cl.Namespace.PublicName, err)
		}
	}

	// Подготавливаем информацию по типам индексов
	for indexNum := range cl.Indexes {
		if len(cl.Indexes[indexNum].Fields) > 1 {
			cl.Indexes[indexNum].Type = cl.Indexes[indexNum].Name + "IndexType"
		} else {
			cl.Indexes[indexNum].Type = string(cl.Fields[cl.Indexes[indexNum].Fields[0]].Format)
		}
	}

	return linkObjects, nil
}

func (a *ArGen) prepareFixtureGenerate(cl *ds.RecordPackage, name string) error {
	for _, fo := range cl.FieldsObjectMap {
		linkObjectPackage := *a.packagesParsed[a.packagesLinked[fo.ObjectName]]

		_, err := cl.FindOrAddImport(linkObjectPackage.Namespace.ModuleName, linkObjectPackage.Namespace.PackageName)
		if err != nil {
			return fmt.Errorf("error process `%s` linkObject for package `%s`: %s", fo.ObjectName, cl.Namespace.PublicName, err)
		}
	}

	recordPackage := *a.packagesParsed[a.packagesLinked[name]]

	_, err := cl.FindOrAddImport(recordPackage.Namespace.ModuleName, name)
	if err != nil {
		return fmt.Errorf("error process `%s` add import declaration for package `%s`: %s", name, cl.Namespace.PublicName, err)
	}

	return nil
}

func (a *ArGen) saveGenerateResult(name, dst string, genRes []generator.GenerateFile) error {
	for _, gen := range genRes {
		dirPkg := filepath.Join(dst, gen.Dir)
		dstFileName := filepath.Join(dirPkg, gen.Name)

		// Сохранение результата генерации в файл
		log.Printf("Write package `%s` (%s) into file `%s`", name, dstFileName, dstFileName)

		if err := writeToFile(dirPkg, dstFileName, gen.Data); err != nil {
			return &arerror.ErrGeneratorFile{Name: name, Backend: gen.Backend, Filename: dstFileName, Err: err}
		}

		// Удаляем из "лишних" фалов то, что перегенерировали
		if _, ex := a.fileToRemove[dstFileName]; ex {
			log.Printf("Replace file: %s", dstFileName)
			delete(a.fileToRemove, dstFileName)
		} else {
			log.Printf("Create file: %s", dstFileName)
		}

		// Удаляем из лишних все директории сгенерированных пакетов
		if _, ex := a.fileToRemove[dirPkg]; ex {
			log.Printf("Replace dir: %s", dirPkg)
			delete(a.fileToRemove, dirPkg)
		}
	}

	return nil
}

// Процесс генерации пакетов по подготовленным данным
func (a *ArGen) generate() error {
	metadata := generator.MetaData{
		AppInfo: a.appInfo.String(),
	}
	// Запускаем цикл с проходом по всем полученным файлам для генерации
	// результирующих пакетов
	for name, cl := range a.packagesParsed {
		// Подготовка информации по ссылкам на другие пакеты
		linkObjects, err := a.prepareGenerate(cl)
		if err != nil {
			return fmt.Errorf("prepare generate error: %s", err)
		}

		// Процесс генерации
		genRes, genErr := generator.Generate(a.appInfo.String(), *cl, linkObjects)
		if genErr != nil {
			return fmt.Errorf("generate error: %s", genErr)
		}

		if err := a.saveGenerateResult(name, a.dst, genRes); err != nil {
			return fmt.Errorf("error save result: %w", err)
		}

		metadata.Namespaces = append(metadata.Namespaces, cl)
	}

	genRes, genErr := generator.GenerateMeta(metadata)
	if genErr != nil {
		return fmt.Errorf("generate meta error: %s", genErr)
	}

	if err := a.saveGenerateResult("meta", a.dst, genRes); err != nil {
		return fmt.Errorf("error save meta result: %w", err)
	}

	if a.skipGenerateFixture() {
		return nil
	}

	// Генерация пакета со сторами фикстур для тестов
	err := a.prepareFixturesStorage()
	if err != nil {
		return fmt.Errorf("prepare fixture store error: %s", err)
	}

	for name, cl := range a.packagesParsed {
		// Подготовка информации по ссылкам на другие пакеты
		err := a.prepareFixtureGenerate(cl, name)
		if err != nil {
			return fmt.Errorf("prepare generate %s fixture store error: %w", name, err)
		}

		// Процесс генерации
		genRes, genErr := generator.GenerateFixture(a.appInfo.String(), *cl, name, a.pkgFixture)
		if genErr != nil {
			return fmt.Errorf("generate %s fixture store error: %w", name, genErr)
		}

		if err := a.saveGenerateResult(name, a.dstFixture, genRes); err != nil {
			return fmt.Errorf("error save generated %s fixture result: %w", name, err)
		}
	}

	return nil
}

// Основная функция запускающая конвеер на выполнение
// всех этапов генерации
// - парсинг
// - обогащение перед проверкой
// - Проверка
// - сбор существующих файлов
// - генерация
// - очистка "лишних" файлов
func (a *ArGen) Run() error {
	// парсим декларацию сущностей
	if err := a.parse(); err != nil {
		return err
	}

	// Подготавливаем данные для проверки
	if err := a.prepareCheck(); err != nil {
		return fmt.Errorf("can't prepare files: %s", err)
	}

	// Проверка информации полученной из декларации на консистентность и валидность
	checkErr := checker.Check(a.packagesParsed, a.packagesLinked)
	if checkErr != nil {
		return fmt.Errorf("error check repository after parse: %s", checkErr)
	}

	if err := a.generate(); err != nil {
		return fmt.Errorf("error generate: %w", err)
	}

	// Очищаем лишние файлы
	for name := range a.fileToRemove {
		log.Printf("Drop file `%s`\n", name)
		os.Remove(name)
	}

	return nil
}

// Создание директории для пакета и запись пакета на диск
func writeToFile(dirPkg string, dstFileName string, data []byte) error {
	if !strings.HasPrefix(dstFileName, dirPkg) {
		return fmt.Errorf("dstFileName must be into dstDir")
	}

	if _, err := os.Stat(dirPkg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err = os.Mkdir(dirPkg, 0700); err != nil {
				return fmt.Errorf("error create dir while save to file: %w", err)
			}
		} else {
			return fmt.Errorf("save generated package to file error: %w", err)
		}
	}

	dstFile, err := os.Create(dstFileName) //nolint:gosec
	if err != nil {
		return fmt.Errorf("error create file: %w", err)
	}

	_, err = dstFile.Write(data)
	if err != nil {
		return fmt.Errorf("error write to file: %w", err)
	}

	dstFile.Close()

	return nil
}

// Сункция обрабатывает все декларации в папке src
// результат парсинга складывает в packagesParsed
func (a *ArGen) parse() error {
	for _, srcFile := range a.srcEntry {
		if !srcFile.Type().IsRegular() {
			return fmt.Errorf("error declaration file `%s`. File in model declaration dir must be regular", srcFile.Name())
		}

		srcFileName := filepath.Join(a.src, srcFile.Name())
		source := srcFile.Name()

		// Создаём новую запись для очередного файла
		// при создании проверяются дубликаты деклараций
		rc, err := a.addRecordPackage(source[:len(source)-3])
		if err != nil {
			return fmt.Errorf("error model(%s) parse: %s", srcFileName, err)
		}

		// Запускаем процесс парсинга
		if err := parser.Parse(srcFileName, rc); err != nil {
			return fmt.Errorf("error parse declaration: %w", err)
		}
	}

	return nil
}

// Регулярное выражение для проверки пути
// Путь участвует в построении пути для импорта, по этому
// нельзя использовать точки, но можно относительно текущей диры
var rxPathValidator = regexp.MustCompile(`^[^\.]`)

// Подготовка рабочих каталогов, чтение деклараций
// Если каталог для генерации существует то проверяется наличие файла .argen
// что бы случайно не удалить какие то файлы после генерации
// Если каталога нет, то он создаётся с файлом .argen для будущих перегенераций
func (a *ArGen) prepareDir() error {
	var err error

	if !rxPathValidator.MatchString(a.src) {
		return fmt.Errorf("invalid path repository declaration")
	}

	if !rxPathValidator.MatchString(a.dst) {
		return fmt.Errorf("invaliv path repository generation")
	}

	a.srcEntry, err = os.ReadDir(a.src)
	if err != nil {
		return fmt.Errorf("error open dir `%s` with repository declaration: %w", a.src, err)
	}

	// Проверка существования каталога для генерации, если нет то создаём
	a.dstEntry, err = os.ReadDir(a.dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("error open dir `%s` for repository generation: %w", a.dst, err)
		}

		err := os.Mkdir(a.dst, 0750)
		if err != nil {
			return fmt.Errorf("error create dir `%s` for repository generation: %w", a.dst, err)
		}

		if err := os.WriteFile(filepath.Join(a.dst, ".argen"), []byte("DO NOT DELETE THIS FILE"), 0600); err != nil {
			return fmt.Errorf("error create spec file `.argen` for repository generation: %w", err)
		}

		a.dstEntry = []fs.DirEntry{}
	} else {
		file, err := os.Open(filepath.Join(a.dst, ".argen"))
		if err != nil {
			return fmt.Errorf("destination directory not empty and hasn't .argen special file")
		}

		file.Close()
	}

	return nil
}

// Подготавливает файлы стораджей для сгенерированных сторов фикстур
func (a *ArGen) prepareFixturesStorage() error {
	var err error

	if !rxPathValidator.MatchString(a.dstFixture) {
		return fmt.Errorf("invaliv path for fixture generation")
	}

	storePath := filepath.Join(a.dstFixture, "data")
	// Проверка существования папки для хранилища фикстур, если нет то создаём
	_, err = os.ReadDir(storePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("error open dir `%s` for fixture storage: %w", storePath, err)
		}

		err = os.MkdirAll(storePath, 0750)
		if err != nil {
			return fmt.Errorf("error create dir `%s` for fixture storage: %w", storePath, err)
		}
	}
	// Для всех генерируемых сущностей создаем файлы для хранения фикстур, если они еще не существуют
	for name := range a.packagesParsed {
		for _, fixtureType := range []string{"", "_update", "_insert_replace"} {
			storagePath := filepath.Join(storePath, name+fixtureType+".yaml")

			if _, err = os.Stat(storagePath); os.IsNotExist(err) {
				if err = os.WriteFile(storagePath, nil, fs.ModePerm); err != nil {
					return fmt.Errorf("error create storage file for %s fixture storage: %w", name, err)
				}
			}

			if err != nil {
				return fmt.Errorf("error check file  for %s fixture storage: %w", name, err)
			}
		}
	}

	return nil
}

// Получение списка существующих пакетов в каталоге для генерации
// Необходимо для составления списка пакетов на удаление после генерации
func (a *ArGen) getExists() ([]string, error) {
	existsFile := []string{}

	// Проходим по всем каталогам результирующей директории
	// По сути это пакеты, которые были сгенерированы в прошлый раз
	for _, dstFile := range a.dstEntry {
		if dstFile.Name() == ".argen" {
			continue
		}

		if dstFile.Name() == "repository.go" {
			continue
		}

		if !dstFile.Type().IsDir() {
			return nil, fmt.Errorf("destination folder can contain only dirs. `%s` not a dir", dstFile.Name())
		}

		dstRepoDir := filepath.Join(a.dst, dstFile.Name())

		existsFile = append(existsFile, dstRepoDir)

		goFiles, err := os.ReadDir(dstRepoDir)
		if err != nil {
			return nil, fmt.Errorf("can'r read destination repository folder(%s): %s", dstFile.Name(), err)
		}

		for _, goFile := range goFiles {
			if !strings.HasSuffix(dstFile.Name(), ".go") {
				if goFile.Type().IsDir() || !goFile.Type().IsRegular() {
					return nil, fmt.Errorf("destination repository folder can contain only go files. `%s` is not a go-file", goFile.Name())
				}

				existsFile = append(existsFile, filepath.Join(dstRepoDir, goFile.Name()))
			}
		}
	}

	return existsFile, nil
}
