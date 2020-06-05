package gateway

import (
	"strconv"
	"sync"

	"github.com/mediocregopher/radix/v3"
)

// ShardStore represents a generic structure that can store information about a shard
type ShardStore interface {
	GetSeq(shardID uint) (seq uint, err error)
	SetSeq(shardID uint, seq uint) error
	GetSession(shardID uint) (session string, err error)
	SetSession(shardID uint, session string) error
}

// LocalShardStore stores shard information in memory
type LocalShardStore struct {
	seqMux     *sync.RWMutex
	sessionMux *sync.RWMutex

	seqs     map[uint]uint
	sessions map[uint]string
}

// NewLocalShardStore initializes a local shard store with the necessary state
func NewLocalShardStore() *LocalShardStore {
	return &LocalShardStore{
		seqMux:     &sync.RWMutex{},
		sessionMux: &sync.RWMutex{},
		seqs:       make(map[uint]uint),
		sessions:   make(map[uint]string),
	}
}

// GetSeq gets the current sequence of the given shard
func (s *LocalShardStore) GetSeq(shardID uint) (seq uint, err error) {
	s.seqMux.RLock()
	defer s.seqMux.RUnlock()

	seq = s.seqs[shardID]
	return
}

// SetSeq sets the current sequence of the given shard, ignoring values that are less than the current value
func (s *LocalShardStore) SetSeq(shardID uint, seq uint) error {
	s.seqMux.Lock()
	defer s.seqMux.Unlock()

	if seq > s.seqs[shardID] {
		s.seqs[shardID] = seq
	}
	return nil
}

// GetSession gets the session identifier for the given shard
func (s *LocalShardStore) GetSession(shardID uint) (session string, err error) {
	s.sessionMux.RLock()
	defer s.sessionMux.RUnlock()

	session = s.sessions[shardID]
	return
}

// SetSession sets the session identifier for the given shard
func (s *LocalShardStore) SetSession(shardID uint, session string) error {
	s.sessionMux.Lock()
	defer s.sessionMux.Unlock()

	s.sessions[shardID] = session
	return nil
}

var setMax = radix.NewEvalScript(1, `
local current = tonumber(redis.call("GET", KEYS[1]))
if current == nil then current = 0 end
if tonumber(ARGV[1]) > current then return redis.call("SET", KEYS[1], ARGV[1]) end
return nil
`)

// RedisShardStore stores information about shards in Redis
type RedisShardStore struct {
	Redis  radix.Client
	Prefix string
}

// GetSeq gets the current sequence of the given shard
func (s *RedisShardStore) GetSeq(shardID uint) (seq uint, err error) {
	err = s.Redis.Do(radix.Cmd(&seq, "GET", s.shardKey(shardID)+"seq"))
	return
}

// SetSeq sets the current sequence of the given shard, ignoring values that are less than the current value
func (s *RedisShardStore) SetSeq(shardID uint, seq uint) error {
	return s.Redis.Do(setMax.Cmd(nil, s.shardKey(shardID)+"seq", strconv.FormatUint(uint64(seq), 10)))
}

// GetSession gets the session identifier for the given shard
func (s *RedisShardStore) GetSession(shardID uint) (session string, err error) {
	err = s.Redis.Do(radix.Cmd(&session, "GET", s.shardKey(shardID)+"session"))
	return
}

// SetSession sets the session identifier for the given shard
func (s *RedisShardStore) SetSession(shardID uint, session string) error {
	return s.Redis.Do(radix.Cmd(nil, "SET", s.shardKey(shardID)+"session", session))
}

func (s *RedisShardStore) shardKey(shardID uint) string {
	return s.Prefix + strconv.FormatUint(uint64(shardID), 10)
}
