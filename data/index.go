package data

const BlockSize = 500

type RootItem struct {
	Crc   uint32
	Index uint32
}

type Root struct {
	Version uint16
	Items   [BlockSize]RootItem
}

type Entry struct {
}

type BlockItem struct {
	Crc uint32
}

type Block struct {
}

func test() {

}
