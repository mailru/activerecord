package iproto

const (
	headerLen = 3 * 4 // 3 * uint32
)

type Header struct {
	Msg  uint32
	Len  uint32
	Sync uint32
}

type Packet struct {
	Header Header
	Data   []byte
}
