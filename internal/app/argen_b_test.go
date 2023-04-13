package app_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mailru/activerecord/internal/app"
	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/internal/pkg/testutil"
	"github.com/mailru/activerecord/pkg/octopus"
)

func TestInit(t *testing.T) {
	tempDirs := testutil.InitTmps()
	defer tempDirs.Defer()

	type args struct {
		ctx     context.Context
		appInfo ds.AppInfo
		dirFlag uint32
		modName string
	}
	tests := []struct {
		name    string
		args    args
		wantNil bool
		wantErr bool
	}{
		{
			name: "Non existing src and dst dirs",
			args: args{
				ctx:     context.Background(),
				appInfo: testutil.TestAppInfo,
				dirFlag: testutil.NonExistsSrcDir | testutil.NonExistsDstDir,
				modName: "github.com/mailru/activerecord",
			},
			wantNil: true,
			wantErr: true,
		},
		{
			name: "non existing dst dir",
			args: args{
				ctx:     context.Background(),
				appInfo: testutil.TestAppInfo,
				dirFlag: testutil.NonExistsDstDir,
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non existing src dir",
			args: args{
				ctx:     context.Background(),
				appInfo: testutil.TestAppInfo,
				dirFlag: testutil.NonExistsSrcDir,
			},
			wantNil: true,
			wantErr: true,
		},
		{
			name: "empty src dir",
			args: args{
				ctx:     context.Background(),
				appInfo: testutil.TestAppInfo,
				dirFlag: testutil.EmptyDstDir,
			},
			wantNil: false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcDir, dstDir, err := tempDirs.CreateDirs(tt.args.dirFlag)
			if err != nil {
				t.Errorf("can't prepare temp dirs: %s", err)
				return
			}

			got, err := app.Init(tt.args.ctx, &tt.args.appInfo, srcDir, dstDir, "", tt.args.modName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, res = %+v, wantErr %v", err, got, tt.wantErr)
				return
			}
			if (got == nil) != tt.wantNil {
				t.Errorf("Init() = %+v, wantNil %v", got, tt.wantNil)
				return
			}
		})
	}
}

func TestArGen_Run(t *testing.T) {
	tempDirs := testutil.InitTmps()
	defer tempDirs.Defer()

	srcRoot, errTmpDir := tempDirs.AddTempDir()
	if errTmpDir != nil {
		t.Errorf("can't prepare root tmp dir: %s", errTmpDir)
		return
	}

	src := filepath.Join(srcRoot, "model/repository/decl")
	dst := filepath.Join(srcRoot, "model/repository/argen")

	if err := os.MkdirAll(src, 0700); err != nil {
		t.Errorf("can't prepare test tmp dir: %s", err)
		return
	}

	textTestPkg := `package repository

	//ar:serverHost:127.0.0.1;serverPort:11111;serverTimeout:500
	//ar:namespace:2
	//ar:backend:octopus
	type FieldsFoo struct {
		Field1    int  ` + "`" + `ar:"size:5"` + "`" + `
		Field2    string  ` + "`" + `ar:"size:5"` + "`" + `
	}

	type (
		IndexesFoo struct {
			Field1Field2 bool ` + "`" + `ar:"fields:Field1,Field2;primary_key"` + "`" + `
		}
		IndexPartsFoo struct {
			Field1Part bool ` + "`" + `ar:"index:Field1Field2;fieldnum:1;selector:SelectByField1"` + "`" + `
		}
	)

	`
	fieldValue := []byte{0x0A, 0x00, 0x00, 0x00}
	fieldValueStr := strconv.FormatUint(uint64(binary.LittleEndian.Uint32(fieldValue)), 10)
	namespace := []byte{0x02, 0x00, 0x00, 0x00}
	insertFlags := []byte{0x03, 0x00, 0x00, 0x00}
	replaceFlags := []byte{0x05, 0x00, 0x00, 0x00}
	insertTupleCardinality := []byte{0x02, 0x00, 0x00, 0x00}
	insertTupleFields := append([]byte{0x04}, fieldValue...) //len + Field1
	insertTupleFields = append(insertTupleFields, []byte{0x00}...)

	insertReq := append(namespace, insertFlags...)
	insertReq = append(insertReq, insertTupleCardinality...)
	insertReq = append(insertReq, insertTupleFields...)

	replaceReq := append(namespace, replaceFlags...)
	replaceReq = append(replaceReq, insertTupleCardinality...)
	replaceReq = append(replaceReq, insertTupleFields...)

	insertMsg := octopus.RequestTypeInsert // Insert or Replace

	responseSuccess := []byte{0x00, 0x00, 0x00, 0x00}
	responseErrorDuplicate := []byte{0x02, 0x20, 0x00, 0x00}
	insertRespCount := []byte{0x01, 0x00, 0x00, 0x00}
	insertFqTupleSize := []byte{0x09, 0x00, 0x00, 0x00} //sizeof byte

	successInsertResponse := append(responseSuccess, insertRespCount...)
	successInsertResponse = append(successInsertResponse, insertFqTupleSize...)
	successInsertResponse = append(successInsertResponse, insertTupleCardinality...)
	successInsertResponse = append(successInsertResponse, insertTupleFields...)

	duplicateInsertResponse := append(responseErrorDuplicate, []byte("Duplicate key")...)

	repositoryName := "foo"

	if err := os.WriteFile(filepath.Join(src, repositoryName+".go"), []byte(textTestPkg), 0600); err != nil {
		t.Errorf("can't write test file: %s", err)
		return
	}

	emptySrc, emptyDst, err := tempDirs.CreateDirs(testutil.NonExistsDstDir)
	if err != nil {
		t.Errorf("can't initialize dirs: %s", err)
		return
	}

	type argsRun struct {
		testGoMain string
		fixtures   []octopus.FixtureType
	}

	type initArgs struct {
		ctx        context.Context
		appInfo    ds.AppInfo
		srcDir     string
		dstDir     string
		dstFixture string
		root       string
		modName    string
	}

	tests := []struct {
		name     string
		initArgs initArgs
		runArgs  []argsRun
		wantErr  bool
	}{
		{
			name: "run on empty src dir",
			initArgs: initArgs{
				ctx:     context.Background(),
				appInfo: testutil.TestAppInfo,
				srcDir:  emptySrc,
				dstDir:  emptyDst,
				modName: "github.com/mailru/activerecord",
			},
			wantErr: false,
		},
		{
			name: "simplePkg full cycle",
			initArgs: initArgs{
				ctx:     context.Background(),
				appInfo: testutil.TestAppInfo,
				srcDir:  src,
				dstDir:  dst,
				root:    srcRoot,
			},
			wantErr: false,
			runArgs: []argsRun{
				{
					testGoMain: `fooRepo := ` + repositoryName + `.New(ctx)
					fooRepo.SetField1(` + fieldValueStr + `)
						err := fooRepo.Insert(ctx)
						if err != nil {
							log.Fatal(err)
						}
						err = fooRepo.Replace(ctx)
						if err != nil {
							log.Fatal(err)
						}`,
					fixtures: []octopus.FixtureType{
						octopus.CreateFixture(1, uint8(insertMsg), insertReq, successInsertResponse, nil),
						octopus.CreateFixture(2, uint8(insertMsg), replaceReq, successInsertResponse, nil),
					},
				},
				{
					testGoMain: `fooRepo := ` + repositoryName + `.New(ctx)
					fooRepo.SetField1(` + fieldValueStr + `)
						err := fooRepo.Insert(ctx)
						if err == nil {
							log.Fatal("Error test duplicate")
						}`,
					fixtures: []octopus.FixtureType{
						octopus.CreateFixture(1, uint8(insertMsg), insertReq, duplicateInsertResponse, nil),
					},
				},
			},
		},
	}

	testModuleName := "github.com/foo/bar/baz/test.git"
	srcPath := testutil.GetPathToSrc()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := app.Init(tt.initArgs.ctx, &tt.initArgs.appInfo, tt.initArgs.srcDir, tt.initArgs.dstDir, tt.initArgs.dstFixture, tt.initArgs.modName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ArGen.Init() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err := got.Run(); (err != nil) != tt.wantErr {
				t.Errorf("ArGen.Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.runArgs != nil && len(tt.runArgs) > 0 {
				gomod := `module ` + testModuleName + `

go 1.19

require (
	github.com/mailru/activerecord v0.9.3
)

replace github.com/mailru/activerecord => ` + srcPath

				modFile := filepath.Join(tt.initArgs.root, "/go.mod")
				if err := os.WriteFile(modFile, []byte(gomod), 0600); err != nil {
					t.Fatalf(fmt.Sprintf("can't write test script: %s", err))
					return
				}

				oms, err := octopus.InitMockServer(octopus.WithHost("127.0.0.1", "11111"))
				if err != nil {
					t.Fatalf(fmt.Sprintf("can't init octopus server: %s", err))
					return
				}

				err = oms.Start()
				if err != nil {
					t.Fatalf("Error start octopusMock %s", err)
					return
				}

				defer func() {
					errStop := oms.Stop()
					if errStop != nil {
						t.Fatalf("Error stop octopusMock %s", errStop)
					}
				}()

				for _, gr := range tt.runArgs {
					main := `package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mailru/activerecord/pkg/activerecord"
	"` + testModuleName + `/model/repository/argen/` + repositoryName + `"
)

func main() {
	ctx := context.Background()
	log.Printf("Start")
	activerecord.InitActiveRecord()
	` + gr.testGoMain + `
	//activerecord.ConnectionCacher().CloseConnection(ctx)
	fmt.Print("OK")
}
`
					runFile := filepath.Join(tt.initArgs.root, "/main.go")
					if err = os.WriteFile(runFile, []byte(main), 0600); err != nil {
						t.Fatalf(fmt.Sprintf("can't write test script: %s", err))
						return
					}

					oms.SetFixtures(gr.fixtures)

					if _, err = os.Stat(filepath.Join(tt.initArgs.root, "model/repository/argen/foo")); errors.Is(err, os.ErrNotExist) {
						t.Error("repository file not generated or has invalid name")
						return
					}

					cmd := exec.Command("go", "mod", "tidy")
					cmd.Dir = tt.initArgs.root
					out, err := cmd.CombinedOutput()
					if err != nil {
						t.Fatalf("error run tidy:\n - out: %s\n - err: %s", out, err)
					}

					cmd = exec.Command("go", "run", runFile)
					cmd.Dir = tt.initArgs.root
					out, err = cmd.Output()

					if err != nil {
						t.Fatalf("Error exec generated model: %s", err.(*exec.ExitError).Stderr)
					}

					if string(out) != "OK" {
						t.Fatalf("Exec generated model has bad stdout: %s", out)
					}
				}
			}
		})
	}
}
