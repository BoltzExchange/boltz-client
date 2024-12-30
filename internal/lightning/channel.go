package lightning

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type ChanId uint64

func NewChanIdFromString(chanId string) (ChanId, error) {
	if chanId == "" {
		return 0, nil
	}
	lnd, err := strconv.ParseInt(chanId, 10, 64)
	if err == nil {
		return ChanId(lnd), nil
	}
	split := strings.Split(chanId, "x")
	if len(split) == 3 {
		var blockHeight, txIndex, txPosition uint64
		_, err := fmt.Sscanf(chanId, "%dx%dx%d", &blockHeight, &txIndex, &txPosition)
		if err == nil {
			return ChanId((blockHeight << 40) | (txIndex << 16) | txPosition), nil
		}
	}
	return 0, errors.New("invalid channel id")
}

func (chanId ChanId) ToCln() string {
	blockHeight := uint32(chanId >> 40)
	txIndex := uint32(chanId>>16) & 0xFFFFFF
	txPosition := uint16(chanId)
	return fmt.Sprintf("%dx%dx%d", blockHeight, txIndex, txPosition)
}

func (chanId ChanId) ToLnd() uint64 {
	return uint64(chanId)
}

func (chanId ChanId) String() string {
	if chanId == 0 {
		return "0"
	}
	return fmt.Sprintf("{cln:%s lnd:%d}", chanId.ToCln(), chanId.ToLnd())
}
