package gateway

import "github.com/spec-tacles/go/types"

// UnknownSendPacket represents a packet to be sent with guild context for determining shard ID
type UnknownSendPacket struct {
	GuildID uint64            `json:"guild_id,string"`
	Packet  *types.SendPacket `json:"packet"`
}
