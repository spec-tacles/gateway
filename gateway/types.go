package gateway

import "github.com/spec-tacles/go/types"

type UnknownSendPacket struct {
	GuildID uint64            `json:"guild_id,string"`
	Packet  *types.SendPacket `json:"packet"`
}
