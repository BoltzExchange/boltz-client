package autoswap

import (
	"errors"
	"github.com/BoltzExchange/boltz-client/lightning"
	"slices"

	"github.com/mennanov/fmutils"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func overwrite(src, dest proto.Message, mask *fieldmaskpb.FieldMask) (proto.Message, error) {
	if src == nil {
		return dest, nil
	}
	if mask == nil {
		return src, nil
	}
	cloned := proto.Clone(dest)
	mask.Normalize()
	if !mask.IsValid(cloned) {
		return nil, errors.New("invalid field mask")
	}
	fmutils.Overwrite(src, cloned, mask.GetPaths())
	return cloned, nil
}

type DismissedChannels map[lightning.ChanId][]string

func (dismissed DismissedChannels) addChannels(chanIds []lightning.ChanId, reason string) {
	if chanIds == nil {
		chanIds = []lightning.ChanId{0}
	}
	for _, chanId := range chanIds {
		if !slices.Contains(dismissed[chanId], reason) {
			dismissed[chanId] = append(dismissed[chanId], reason)
		}
	}
}

func merge[T proto.Message](base, config T) T {
	proto.Merge(base, config)
	return base
}
