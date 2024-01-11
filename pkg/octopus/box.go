package octopus

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/mailru/activerecord/pkg/activerecord"
	"github.com/mailru/activerecord/pkg/iproto/iproto"
)

var DefaultOptionCreator = func(sic activerecord.ShardInstanceConfig) (activerecord.OptionInterface, error) {
	return NewOptions(
		sic.Addr,
		ServerModeType(sic.Mode),
		WithTimeout(sic.Timeout, sic.Timeout),
		WithPoolSize(sic.PoolSize),
		WithPoolLogger(activerecord.IprotoLogger{}),
	)
}

var DefaultConnectionParams = activerecord.MapGlobParam{
	Timeout:  DefaultConnectionTimeout,
	PoolSize: DefaultPoolSize,
}

var ClusterConfigParams = activerecord.NewClusterConfigParameters(DefaultConnectionParams, DefaultOptionCreator, CheckShardInstance)

// Box - возвращает коннектор для БД
// TODO
// - сделать статистику по используемым инстансам
// - прикрутить локальный пингер и исключать недоступные инстансы
func Box(ctx context.Context, shard int, instType activerecord.ShardInstanceType, configPath string, optionCreator func(activerecord.ShardInstanceConfig) (activerecord.OptionInterface, error)) (*Connection, error) {
	if optionCreator == nil {
		optionCreator = DefaultOptionCreator
	}

	clusterInfo, err := activerecord.ConfigCacher().Get(
		ctx,
		configPath,
		DefaultConnectionParams,
		optionCreator,
	)
	if err != nil {
		return nil, fmt.Errorf("can't get cluster %s info: %w", configPath, err)
	}

	if clusterInfo.Shards() < shard {
		return nil, fmt.Errorf("invalid shard num %d, max = %d", shard, clusterInfo.Shards())
	}

	var configBox activerecord.ShardInstance

	switch instType {
	case activerecord.ReplicaInstanceType:
		if !clusterInfo.HasReplicas(shard) {
			return nil, fmt.Errorf("replicas not set")
		}

		configBox = clusterInfo.NextReplica(shard)
	case activerecord.ReplicaOrMasterInstanceType:
		if clusterInfo.HasReplicas(shard) {
			configBox = clusterInfo.NextReplica(shard)
			break
		}

		fallthrough
	case activerecord.MasterInstanceType:
		configBox = clusterInfo.NextMaster(shard)
	}

	conn, err := activerecord.ConnectionCacher().GetOrAdd(configBox, func(options interface{}) (activerecord.ConnectionInterface, error) {
		octopusOpt, ok := options.(*ConnectionOptions)
		if !ok {
			return nil, fmt.Errorf("invalit type of options %T, want Options", options)
		}

		return GetConnection(ctx, octopusOpt)
	})
	if err != nil {
		return nil, fmt.Errorf("error from connectionCacher: %w", err)
	}

	box, ok := conn.(*Connection)
	if !ok {
		return nil, fmt.Errorf("invalid connection type %T, want *octopus.Connection", conn)
	}

	return box, nil
}

func CheckShardInstance(ctx context.Context, instance activerecord.ShardInstance) (activerecord.OptionInterface, error) {
	octopusOpt, ok := instance.Options.(*ConnectionOptions)
	if !ok {
		return nil, fmt.Errorf("invalit type of options %T, want Options", instance.Options)
	}

	var err error
	c := activerecord.ConnectionCacher().Get(instance)
	if c == nil {
		c, err = GetConnection(ctx, octopusOpt)
		if err != nil {
			return nil, fmt.Errorf("error from connectionCacher: %w", err)
		}
	}

	conn, ok := c.(*Connection)
	if !ok {
		return nil, fmt.Errorf("invalid connection type %T, want *octopus.Connection", conn)
	}

	if len(conn.pool.Online()) == 0 {
		return nil, fmt.Errorf("no online channels")
	}

	td, err := CallLua(ctx, conn, "box.dostring", "return box.info.status")
	if err != nil {
		return nil, fmt.Errorf("can't get status: %w", err)
	}

	if len(td) == 1 {
		ret := td[0]
		switch string(ret.Data[0]) {
		case "primary":
			octopusOpt.Mode = ModeMaster
		default:
			octopusOpt.Mode = ModeReplica
		}

		return octopusOpt, nil
	}

	return nil, fmt.Errorf("can't parse status: %w", err)
}

func ProcessResp(respBytes []byte, cntFlag CountFlags) ([]TupleData, error) {
	tupleCnt, respData, errResp := UnpackResopnseStatus(respBytes)
	if errResp != nil {
		return nil, fmt.Errorf("error response from box: `%w`", errResp)
	}

	if cntFlag&UniqRespFlag == UniqRespFlag && tupleCnt > 2 {
		return nil, fmt.Errorf("returning more than one tuple: %d", tupleCnt)
	}

	if cntFlag&NeedRespFlag == NeedRespFlag && tupleCnt == 0 {
		return nil, fmt.Errorf("empty tuple")
	}

	rdr := bytes.NewReader(respData)

	var tuplesData []TupleData
	tuplesData = make([]TupleData, 0, tupleCnt)

	for f := 0; f < int(tupleCnt); f++ {
		var tupleSize, fieldCnt, totalFieldLen uint32

		if err := iproto.UnpackUint32(rdr, &tupleSize, iproto.ModeDefault); err != nil {
			return nil, fmt.Errorf("error unpacking tuple '%w'", err)
		}

		if uint32(rdr.Len()) < tupleSize {
			return nil, fmt.Errorf("error tuple(%d) size %d, need %d", f+1, rdr.Len(), tupleSize)
		}

		if err := iproto.UnpackUint32(rdr, &fieldCnt, iproto.ModeDefault); err != nil {
			return nil, fmt.Errorf("error unpack fields cnt in tuple %d: %w", f, err)
		}

		td := TupleData{Cnt: fieldCnt}
		td.Data = make([][]byte, 0, fieldCnt)

		for ff := 0; ff < int(fieldCnt); ff++ {
			var fieldLen uint32

			err := iproto.UnpackUint32(rdr, &fieldLen, iproto.ModeBER)
			if err != nil {
				return nil, fmt.Errorf("error unpack fieldLen(%d) in tuple(%d): '%w'", ff, f, err)
			}

			if totalFieldLen+fieldLen > tupleSize {
				return nil, fmt.Errorf("len fields overflow(%d) in tuple(%d)", totalFieldLen+fieldLen, f)
			}

			totalFieldLen += fieldLen
			td.Data = append(td.Data, respData[len(respData)-rdr.Len():len(respData)-rdr.Len()+int(fieldLen)])

			if _, err := rdr.Seek(int64(fieldLen), io.SeekCurrent); err != nil {
				return nil, fmt.Errorf("can't seek^ %w", err)
			}
		}

		tuplesData = append(tuplesData, td)
	}

	if rdr.Len() > 0 {
		return nil, fmt.Errorf("extra data in resp: '%X'", respData[len(respData)-rdr.Len():])
	}

	return tuplesData, nil
}

func PackInsertReplace(ns uint32, insertMode InsertMode, tuple [][]byte) []byte {
	w := make([]byte, 0, SpaceLen+FlagsLen+FieldNumLen+PackedTupleLen(tuple))

	w = PackSpace(w, ns)
	w = PackRequestFlagsVal(w, true, insertMode)
	w = PackTuple(w, tuple)

	return w
}

func UnpackInsertReplace(data []byte) (ns uint32, needRetVal bool, insertMode InsertMode, tuple [][]byte, err error) {
	rdr := bytes.NewReader(data)

	ns, err = UnpackSpace(rdr)
	if err != nil {
		return
	}

	needRetVal, insertMode, err = UnpackRequestFlagsVal(rdr) // Always true, 0, see PackUpdate
	if err != nil {
		err = fmt.Errorf("can't unpack flags: %s", err)
		return
	}

	tuple, err = UnpackTuple(rdr)
	if err != nil {
		err = fmt.Errorf("can't unpack insert tuple: %s", err)
		return
	}

	return
}

func PackSelect(ns, indexnum, offset, limit uint32, keys [][][]byte) []byte {
	w := make([]byte, 0, SpaceLen+IndexLen+OffsetLen+LimitLen+PackedTuplesLen(keys))

	w = PackSpace(w, ns)
	w = PackIndexNum(w, indexnum)
	w = PackOffset(w, offset)
	w = PackLimit(w, limit)
	w = PackTuples(w, keys)

	return w
}

func PackUpdate(ns uint32, primaryKey [][]byte, updateOps []Ops) []byte {
	w := make([]byte, 0, SpaceLen+FlagsLen+PackedKeyLen(primaryKey)+PackedUpdateOpsLen(updateOps))

	w = PackSpace(w, ns)
	w = PackRequestFlagsVal(w, true, 0)
	w = PackKey(w, primaryKey)

	if len(updateOps) != 0 {
		w = iproto.PackUint32(w, uint32(len(updateOps)), iproto.ModeDefault)

		for _, op := range updateOps {
			w = iproto.PackUint32(w, op.Field, iproto.ModeDefault)
			w = append(w, byte(op.Op))
			w = iproto.PackBytes(w, op.Value, iproto.ModeBER)
		}
	}

	return w
}

func UnpackUpdate(data []byte) (ns uint32, primaryKey [][]byte, updateOps []Ops, err error) {
	rdr := bytes.NewReader(data)

	ns, err = UnpackSpace(rdr)
	if err != nil {
		return
	}

	_, _, err = UnpackRequestFlagsVal(rdr) // Always true, 0, see PackUpdate
	if err != nil {
		err = fmt.Errorf("can't unpack flags: %s", err)
		return
	}

	primaryKey, err = UnpackKey(rdr)
	if err != nil {
		err = fmt.Errorf("can't unpack PK: %s", err)
		return
	}

	if rdr.Len() != 0 {
		numUpdate := uint32(0)

		err = iproto.UnpackUint32(rdr, &numUpdate, iproto.ModeDefault)
		if err != nil {
			err = fmt.Errorf("can't unpack updateOps len")
			return
		}

		updateOps = make([]Ops, 0, numUpdate)

		for f := 0; f < int(numUpdate); f++ {
			op := Ops{}

			err = iproto.UnpackUint32(rdr, &op.Field, iproto.ModeDefault)
			if err != nil {
				err = fmt.Errorf("can't unpack field name from updateops (%d): %s", f, err)
				return
			}

			opCode, errOp := rdr.ReadByte()
			if err != nil {
				err = fmt.Errorf("can't unpack opCode from updateops (%d): %s", f, errOp)
				return
			}

			op.Op = OpCode(opCode)

			err = iproto.UnpackBytes(rdr, &op.Value, iproto.ModeBER)
			if err != nil {
				err = fmt.Errorf("can't unpack field value from updateops (%d): %s", f, err)
				return
			}

			updateOps = append(updateOps, op)
		}
	}

	return
}

func PackDelete(ns uint32, primaryKey [][]byte) []byte {
	w := make([]byte, 0, SpaceLen+FlagsLen+PackedKeysLen(primaryKey))

	w = PackSpace(w, ns)
	w = PackRequestFlagsVal(w, true, 0)
	w = PackKey(w, primaryKey)

	return w
}

func UnpackDelete(data []byte) (ns uint32, primaryKey [][]byte, err error) {
	rdr := bytes.NewReader(data)

	ns, err = UnpackSpace(rdr)
	if err != nil {
		return
	}

	_, _, err = UnpackRequestFlagsVal(rdr) // Always true, 0, see PackDelete
	if err != nil {
		err = fmt.Errorf("can't unpack flags: %s", err)
		return
	}

	primaryKey, err = UnpackKey(rdr)
	if err != nil {
		err = fmt.Errorf("can't unpack PK: %s", err)
		return
	}

	return
}

func PackLua(name string, args ...string) []byte {
	w := iproto.PackUint32([]byte{}, 0, iproto.ModeDefault)         // Всегда константа 0
	w = iproto.PackBytes(w, []byte(name), iproto.ModeBER)           // Название lua процедуры с длинной в BER формате
	w = iproto.PackUint32(w, uint32(len(args)), iproto.ModeDefault) // Количество аргументов

	for _, arg := range args {
		w = iproto.PackBytes(w, []byte(arg), iproto.ModeBER) // Аргументы с длинной в BER вормате
	}

	return w
}

func UnpackLua(data []byte) (name string, args [][]byte, err error) {
	r := bytes.NewReader(data)

	var v uint32

	if err = iproto.UnpackUint32(r, &v, iproto.ModeDefault); err != nil || v != 0 {
		return name, args, fmt.Errorf("can't unpack as call lua procedure: %w", err)
	}

	var procName []byte

	if err = iproto.UnpackBytes(r, &procName, iproto.ModeBER); err != nil {
		return name, args, fmt.Errorf("can't unpack lua procedure name: %w", err)
	}

	if args, err = UnpackTuple(r); err != nil {
		err = fmt.Errorf("can't unpack lua procedure args: %s", err)
		return
	}

	return string(procName), args, nil

}
