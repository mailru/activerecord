// Code generated by mockery v2.20.0. DO NOT EDIT.

package octopus

import mock "github.com/stretchr/testify/mock"

// MockMockServerLogger is an autogenerated mock type for the MockServerLogger type
type MockMockServerLogger struct {
	mock.Mock
}

type MockMockServerLogger_Expecter struct {
	mock *mock.Mock
}

func (_m *MockMockServerLogger) EXPECT() *MockMockServerLogger_Expecter {
	return &MockMockServerLogger_Expecter{mock: &_m.Mock}
}

// Debug provides a mock function with given fields: fmt, args
func (_m *MockMockServerLogger) Debug(fmt string, args ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, fmt)
	_ca = append(_ca, args...)
	_m.Called(_ca...)
}

// MockMockServerLogger_Debug_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Debug'
type MockMockServerLogger_Debug_Call struct {
	*mock.Call
}

// Debug is a helper method to define mock.On call
//   - fmt string
//   - args ...interface{}
func (_e *MockMockServerLogger_Expecter) Debug(fmt interface{}, args ...interface{}) *MockMockServerLogger_Debug_Call {
	return &MockMockServerLogger_Debug_Call{Call: _e.mock.On("Debug",
		append([]interface{}{fmt}, args...)...)}
}

func (_c *MockMockServerLogger_Debug_Call) Run(run func(fmt string, args ...interface{})) *MockMockServerLogger_Debug_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]interface{}, len(args)-1)
		for i, a := range args[1:] {
			if a != nil {
				variadicArgs[i] = a.(interface{})
			}
		}
		run(args[0].(string), variadicArgs...)
	})
	return _c
}

func (_c *MockMockServerLogger_Debug_Call) Return() *MockMockServerLogger_Debug_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockMockServerLogger_Debug_Call) RunAndReturn(run func(string, ...interface{})) *MockMockServerLogger_Debug_Call {
	_c.Call.Return(run)
	return _c
}

// DebugCallRequest provides a mock function with given fields: procName, args, fixtures
func (_m *MockMockServerLogger) DebugCallRequest(procName string, args [][]byte, fixtures ...CallMockFixture) {
	_va := make([]interface{}, len(fixtures))
	for _i := range fixtures {
		_va[_i] = fixtures[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, procName, args)
	_ca = append(_ca, _va...)
	_m.Called(_ca...)
}

// MockMockServerLogger_DebugCallRequest_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DebugCallRequest'
type MockMockServerLogger_DebugCallRequest_Call struct {
	*mock.Call
}

// DebugCallRequest is a helper method to define mock.On call
//   - procName string
//   - args [][]byte
//   - fixtures ...CallMockFixture
func (_e *MockMockServerLogger_Expecter) DebugCallRequest(procName interface{}, args interface{}, fixtures ...interface{}) *MockMockServerLogger_DebugCallRequest_Call {
	return &MockMockServerLogger_DebugCallRequest_Call{Call: _e.mock.On("DebugCallRequest",
		append([]interface{}{procName, args}, fixtures...)...)}
}

func (_c *MockMockServerLogger_DebugCallRequest_Call) Run(run func(procName string, args [][]byte, fixtures ...CallMockFixture)) *MockMockServerLogger_DebugCallRequest_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]CallMockFixture, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(CallMockFixture)
			}
		}
		run(args[0].(string), args[1].([][]byte), variadicArgs...)
	})
	return _c
}

func (_c *MockMockServerLogger_DebugCallRequest_Call) Return() *MockMockServerLogger_DebugCallRequest_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockMockServerLogger_DebugCallRequest_Call) RunAndReturn(run func(string, [][]byte, ...CallMockFixture)) *MockMockServerLogger_DebugCallRequest_Call {
	_c.Call.Return(run)
	return _c
}

// DebugDeleteRequest provides a mock function with given fields: ns, primaryKey, fixtures
func (_m *MockMockServerLogger) DebugDeleteRequest(ns uint32, primaryKey [][]byte, fixtures ...DeleteMockFixture) {
	_va := make([]interface{}, len(fixtures))
	for _i := range fixtures {
		_va[_i] = fixtures[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ns, primaryKey)
	_ca = append(_ca, _va...)
	_m.Called(_ca...)
}

// MockMockServerLogger_DebugDeleteRequest_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DebugDeleteRequest'
type MockMockServerLogger_DebugDeleteRequest_Call struct {
	*mock.Call
}

// DebugDeleteRequest is a helper method to define mock.On call
//   - ns uint32
//   - primaryKey [][]byte
//   - fixtures ...DeleteMockFixture
func (_e *MockMockServerLogger_Expecter) DebugDeleteRequest(ns interface{}, primaryKey interface{}, fixtures ...interface{}) *MockMockServerLogger_DebugDeleteRequest_Call {
	return &MockMockServerLogger_DebugDeleteRequest_Call{Call: _e.mock.On("DebugDeleteRequest",
		append([]interface{}{ns, primaryKey}, fixtures...)...)}
}

func (_c *MockMockServerLogger_DebugDeleteRequest_Call) Run(run func(ns uint32, primaryKey [][]byte, fixtures ...DeleteMockFixture)) *MockMockServerLogger_DebugDeleteRequest_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]DeleteMockFixture, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(DeleteMockFixture)
			}
		}
		run(args[0].(uint32), args[1].([][]byte), variadicArgs...)
	})
	return _c
}

func (_c *MockMockServerLogger_DebugDeleteRequest_Call) Return() *MockMockServerLogger_DebugDeleteRequest_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockMockServerLogger_DebugDeleteRequest_Call) RunAndReturn(run func(uint32, [][]byte, ...DeleteMockFixture)) *MockMockServerLogger_DebugDeleteRequest_Call {
	_c.Call.Return(run)
	return _c
}

// DebugInsertRequest provides a mock function with given fields: ns, needRetVal, insertMode, tuple, fixtures
func (_m *MockMockServerLogger) DebugInsertRequest(ns uint32, needRetVal bool, insertMode InsertMode, tuple TupleData, fixtures ...InsertMockFixture) {
	_va := make([]interface{}, len(fixtures))
	for _i := range fixtures {
		_va[_i] = fixtures[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ns, needRetVal, insertMode, tuple)
	_ca = append(_ca, _va...)
	_m.Called(_ca...)
}

// MockMockServerLogger_DebugInsertRequest_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DebugInsertRequest'
type MockMockServerLogger_DebugInsertRequest_Call struct {
	*mock.Call
}

// DebugInsertRequest is a helper method to define mock.On call
//   - ns uint32
//   - needRetVal bool
//   - insertMode InsertMode
//   - tuple TupleData
//   - fixtures ...InsertMockFixture
func (_e *MockMockServerLogger_Expecter) DebugInsertRequest(ns interface{}, needRetVal interface{}, insertMode interface{}, tuple interface{}, fixtures ...interface{}) *MockMockServerLogger_DebugInsertRequest_Call {
	return &MockMockServerLogger_DebugInsertRequest_Call{Call: _e.mock.On("DebugInsertRequest",
		append([]interface{}{ns, needRetVal, insertMode, tuple}, fixtures...)...)}
}

func (_c *MockMockServerLogger_DebugInsertRequest_Call) Run(run func(ns uint32, needRetVal bool, insertMode InsertMode, tuple TupleData, fixtures ...InsertMockFixture)) *MockMockServerLogger_DebugInsertRequest_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]InsertMockFixture, len(args)-4)
		for i, a := range args[4:] {
			if a != nil {
				variadicArgs[i] = a.(InsertMockFixture)
			}
		}
		run(args[0].(uint32), args[1].(bool), args[2].(InsertMode), args[3].(TupleData), variadicArgs...)
	})
	return _c
}

func (_c *MockMockServerLogger_DebugInsertRequest_Call) Return() *MockMockServerLogger_DebugInsertRequest_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockMockServerLogger_DebugInsertRequest_Call) RunAndReturn(run func(uint32, bool, InsertMode, TupleData, ...InsertMockFixture)) *MockMockServerLogger_DebugInsertRequest_Call {
	_c.Call.Return(run)
	return _c
}

// DebugSelectRequest provides a mock function with given fields: ns, indexnum, offset, limit, keys, fixtures
func (_m *MockMockServerLogger) DebugSelectRequest(ns uint32, indexnum uint32, offset uint32, limit uint32, keys [][][]byte, fixtures ...SelectMockFixture) {
	_va := make([]interface{}, len(fixtures))
	for _i := range fixtures {
		_va[_i] = fixtures[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ns, indexnum, offset, limit, keys)
	_ca = append(_ca, _va...)
	_m.Called(_ca...)
}

// MockMockServerLogger_DebugSelectRequest_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DebugSelectRequest'
type MockMockServerLogger_DebugSelectRequest_Call struct {
	*mock.Call
}

// DebugSelectRequest is a helper method to define mock.On call
//   - ns uint32
//   - indexnum uint32
//   - offset uint32
//   - limit uint32
//   - keys [][][]byte
//   - fixtures ...SelectMockFixture
func (_e *MockMockServerLogger_Expecter) DebugSelectRequest(ns interface{}, indexnum interface{}, offset interface{}, limit interface{}, keys interface{}, fixtures ...interface{}) *MockMockServerLogger_DebugSelectRequest_Call {
	return &MockMockServerLogger_DebugSelectRequest_Call{Call: _e.mock.On("DebugSelectRequest",
		append([]interface{}{ns, indexnum, offset, limit, keys}, fixtures...)...)}
}

func (_c *MockMockServerLogger_DebugSelectRequest_Call) Run(run func(ns uint32, indexnum uint32, offset uint32, limit uint32, keys [][][]byte, fixtures ...SelectMockFixture)) *MockMockServerLogger_DebugSelectRequest_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]SelectMockFixture, len(args)-5)
		for i, a := range args[5:] {
			if a != nil {
				variadicArgs[i] = a.(SelectMockFixture)
			}
		}
		run(args[0].(uint32), args[1].(uint32), args[2].(uint32), args[3].(uint32), args[4].([][][]byte), variadicArgs...)
	})
	return _c
}

func (_c *MockMockServerLogger_DebugSelectRequest_Call) Return() *MockMockServerLogger_DebugSelectRequest_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockMockServerLogger_DebugSelectRequest_Call) RunAndReturn(run func(uint32, uint32, uint32, uint32, [][][]byte, ...SelectMockFixture)) *MockMockServerLogger_DebugSelectRequest_Call {
	_c.Call.Return(run)
	return _c
}

// DebugUpdateRequest provides a mock function with given fields: ns, primaryKey, updateOps, fixtures
func (_m *MockMockServerLogger) DebugUpdateRequest(ns uint32, primaryKey [][]byte, updateOps []Ops, fixtures ...UpdateMockFixture) {
	_va := make([]interface{}, len(fixtures))
	for _i := range fixtures {
		_va[_i] = fixtures[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ns, primaryKey, updateOps)
	_ca = append(_ca, _va...)
	_m.Called(_ca...)
}

// MockMockServerLogger_DebugUpdateRequest_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DebugUpdateRequest'
type MockMockServerLogger_DebugUpdateRequest_Call struct {
	*mock.Call
}

// DebugUpdateRequest is a helper method to define mock.On call
//   - ns uint32
//   - primaryKey [][]byte
//   - updateOps []Ops
//   - fixtures ...UpdateMockFixture
func (_e *MockMockServerLogger_Expecter) DebugUpdateRequest(ns interface{}, primaryKey interface{}, updateOps interface{}, fixtures ...interface{}) *MockMockServerLogger_DebugUpdateRequest_Call {
	return &MockMockServerLogger_DebugUpdateRequest_Call{Call: _e.mock.On("DebugUpdateRequest",
		append([]interface{}{ns, primaryKey, updateOps}, fixtures...)...)}
}

func (_c *MockMockServerLogger_DebugUpdateRequest_Call) Run(run func(ns uint32, primaryKey [][]byte, updateOps []Ops, fixtures ...UpdateMockFixture)) *MockMockServerLogger_DebugUpdateRequest_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]UpdateMockFixture, len(args)-3)
		for i, a := range args[3:] {
			if a != nil {
				variadicArgs[i] = a.(UpdateMockFixture)
			}
		}
		run(args[0].(uint32), args[1].([][]byte), args[2].([]Ops), variadicArgs...)
	})
	return _c
}

func (_c *MockMockServerLogger_DebugUpdateRequest_Call) Return() *MockMockServerLogger_DebugUpdateRequest_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockMockServerLogger_DebugUpdateRequest_Call) RunAndReturn(run func(uint32, [][]byte, []Ops, ...UpdateMockFixture)) *MockMockServerLogger_DebugUpdateRequest_Call {
	_c.Call.Return(run)
	return _c
}

type mockConstructorTestingTNewMockMockServerLogger interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockMockServerLogger creates a new instance of MockMockServerLogger. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockMockServerLogger(t mockConstructorTestingTNewMockMockServerLogger) *MockMockServerLogger {
	mock := &MockMockServerLogger{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
