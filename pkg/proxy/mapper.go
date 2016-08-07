// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"bytes"
	"hash/crc32"
	"strings"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/proxy/redis"
	"github.com/CodisLabs/codis/pkg/utils/errors"
)

var charmap [256]byte

func init() {
	for i := 0; i < len(charmap); i++ {
		c := byte(i)
		switch {
		case c >= 'A' && c <= 'Z':
			charmap[i] = c
		case c >= 'a' && c <= 'z':
			charmap[i] = c - 'a' + 'A'
		}
	}
}

type OpFlag uint32

func (f OpFlag) IsNotAllow() bool {
	return (f & FlagNotAllow) != 0
}

func (f OpFlag) IsReadOnly() bool {
	const mask = FlagWrite | FlagMayWrite
	return (f & mask) == 0
}

type OpInfo struct {
	Name string
	Flag OpFlag
}

const (
	FlagWrite = 1 << iota
	FlagMayWrite
	FlagNotAllow
)

var opTable = make(map[string]OpInfo, 256)

func init() {
	for _, i := range []OpInfo{
		{"APPEND", FlagWrite},
		{"AUTH", 0},
		{"BGREWRITEAOF", FlagNotAllow},
		{"BGSAVE", FlagNotAllow},
		{"BITCOUNT", 0},
		{"BITOP", FlagWrite | FlagNotAllow},
		{"BITPOS", 0},
		{"BLPOP", FlagWrite | FlagNotAllow},
		{"BRPOP", FlagWrite | FlagNotAllow},
		{"BRPOPLPUSH", FlagWrite | FlagNotAllow},
		{"CLIENT", FlagNotAllow},
		{"COMMAND", 0},
		{"CONFIG", FlagNotAllow},
		{"DBSIZE", FlagNotAllow},
		{"DEBUG", FlagNotAllow},
		{"DECR", FlagWrite},
		{"DECRBY", FlagWrite},
		{"DEL", FlagWrite},
		{"DISCARD", FlagNotAllow},
		{"DUMP", 0},
		{"ECHO", 0},
		{"EVAL", FlagWrite},
		{"EVALSHA", FlagWrite},
		{"EXEC", FlagNotAllow},
		{"EXISTS", 0},
		{"EXPIRE", FlagWrite},
		{"EXPIREAT", FlagWrite},
		{"FLUSHALL", FlagWrite | FlagNotAllow},
		{"FLUSHDB", FlagWrite | FlagNotAllow},
		{"GET", 0},
		{"GETBIT", 0},
		{"GETRANGE", 0},
		{"GETSET", FlagWrite},
		{"HDEL", FlagWrite},
		{"HEXISTS", 0},
		{"HGET", 0},
		{"HGETALL", 0},
		{"HINCRBY", FlagWrite},
		{"HINCRBYFLOAT", FlagWrite},
		{"HKEYS", 0},
		{"HLEN", 0},
		{"HMGET", 0},
		{"HMSET", FlagWrite},
		{"HSCAN", 0},
		{"HSET", FlagWrite},
		{"HSETNX", FlagWrite},
		{"HVALS", 0},
		{"INCR", FlagWrite},
		{"INCRBY", FlagWrite},
		{"INCRBYFLOAT", FlagWrite},
		{"INFO", 0},
		{"KEYS", FlagNotAllow},
		{"LASTSAVE", FlagNotAllow},
		{"LATENCY", FlagNotAllow},
		{"LINDEX", 0},
		{"LINSERT", FlagWrite},
		{"LLEN", 0},
		{"LPOP", FlagWrite},
		{"LPUSH", FlagWrite},
		{"LPUSHX", FlagWrite},
		{"LRANGE", 0},
		{"LREM", FlagWrite},
		{"LSET", FlagWrite},
		{"LTRIM", FlagWrite},
		{"MGET", 0},
		{"MIGRATE", FlagWrite | FlagNotAllow},
		{"MONITOR", FlagNotAllow},
		{"MOVE", FlagWrite | FlagNotAllow},
		{"MSET", FlagWrite},
		{"MSETNX", FlagWrite | FlagNotAllow},
		{"MULTI", FlagNotAllow},
		{"OBJECT", FlagNotAllow},
		{"PERSIST", FlagWrite},
		{"PEXPIRE", FlagWrite},
		{"PEXPIREAT", FlagWrite},
		{"PFADD", FlagWrite},
		{"PFCOUNT", 0},
		{"PFDEBUG", FlagWrite},
		{"PFMERGE", FlagWrite},
		{"PFSELFTEST", 0},
		{"PING", 0},
		{"PSETEX", FlagWrite},
		{"PSUBSCRIBE", FlagNotAllow},
		{"PSYNC", FlagNotAllow},
		{"PTTL", 0},
		{"PUBLISH", FlagNotAllow},
		{"PUBSUB", 0},
		{"PUNSUBSCRIBE", FlagNotAllow},
		{"RANDOMKEY", FlagNotAllow},
		{"RENAME", FlagWrite | FlagNotAllow},
		{"RENAMENX", FlagWrite | FlagNotAllow},
		{"REPLCONF", FlagNotAllow},
		{"RESTORE", FlagWrite | FlagNotAllow},
		{"ROLE", 0},
		{"RPOP", FlagWrite},
		{"RPOPLPUSH", FlagWrite},
		{"RPUSH", FlagWrite},
		{"RPUSHX", FlagWrite},
		{"SADD", FlagWrite},
		{"SAVE", FlagNotAllow},
		{"SCAN", FlagNotAllow},
		{"SCARD", 0},
		{"SCRIPT", FlagNotAllow},
		{"SDIFF", 0},
		{"SDIFFSTORE", FlagWrite},
		{"SELECT", 0},
		{"SET", FlagWrite},
		{"SETBIT", FlagWrite},
		{"SETEX", FlagWrite},
		{"SETNX", FlagWrite},
		{"SETRANGE", FlagWrite},
		{"SHUTDOWN", FlagNotAllow},
		{"SINTER", 0},
		{"SINTERSTORE", FlagWrite},
		{"SISMEMBER", 0},
		{"SLAVEOF", FlagNotAllow},
		{"SLOTSCHECK", FlagNotAllow},
		{"SLOTSDEL", FlagWrite | FlagNotAllow},
		{"SLOTSHASHKEY", 0},
		{"SLOTSINFO", FlagNotAllow},
		{"SLOTSMAPPING", 0},
		{"SLOTSMGRTONE", FlagWrite | FlagNotAllow},
		{"SLOTSMGRTSLOT", FlagWrite | FlagNotAllow},
		{"SLOTSMGRTTAGONE", FlagWrite | FlagNotAllow},
		{"SLOTSMGRTTAGSLOT", FlagWrite | FlagNotAllow},
		{"SLOTSRESTORE", FlagWrite},
		{"SLOTSSCAN", 0},
		{"SLOWLOG", FlagNotAllow},
		{"SMEMBERS", 0},
		{"SMOVE", FlagWrite},
		{"SORT", FlagWrite},
		{"SPOP", FlagWrite},
		{"SRANDMEMBER", 0},
		{"SREM", FlagWrite},
		{"SSCAN", 0},
		{"STRLEN", 0},
		{"SUBSCRIBE", FlagNotAllow},
		{"SUBSTR", 0},
		{"SUNION", 0},
		{"SUNIONSTORE", FlagWrite},
		{"SYNC", FlagNotAllow},
		{"TIME", FlagNotAllow},
		{"TTL", 0},
		{"TYPE", 0},
		{"UNSUBSCRIBE", FlagNotAllow},
		{"UNWATCH", FlagNotAllow},
		{"WATCH", FlagNotAllow},
		{"ZADD", FlagWrite},
		{"ZCARD", 0},
		{"ZCOUNT", 0},
		{"ZINCRBY", FlagWrite},
		{"ZINTERSTORE", FlagWrite},
		{"ZLEXCOUNT", 0},
		{"ZRANGE", 0},
		{"ZRANGEBYLEX", 0},
		{"ZRANGEBYSCORE", 0},
		{"ZRANK", 0},
		{"ZREM", FlagWrite},
		{"ZREMRANGEBYLEX", FlagWrite},
		{"ZREMRANGEBYRANK", FlagWrite},
		{"ZREMRANGEBYSCORE", FlagWrite},
		{"ZREVRANGE", 0},
		{"ZREVRANGEBYLEX", 0},
		{"ZREVRANGEBYSCORE", 0},
		{"ZREVRANK", 0},
		{"ZSCAN", 0},
		{"ZSCORE", 0},
		{"ZUNIONSTORE", FlagWrite},
	} {
		opTable[i.Name] = i
	}
}

var (
	ErrBadMultiBulk = errors.New("bad multi-bulk for command")
	ErrBadOpStrLen  = errors.New("bad command length, too short or too long")
)

const MaxOpStrLen = 64

func getOpInfo(multi []*redis.Resp) (string, OpFlag, error) {
	if len(multi) < 1 {
		return "", 0, errors.Trace(ErrBadMultiBulk)
	}

	var upper [MaxOpStrLen]byte

	var op = multi[0].Value
	if len(op) == 0 || len(op) > len(upper) {
		return "", 0, errors.Trace(ErrBadOpStrLen)
	}
	for i := 0; i < len(op); i++ {
		if c := charmap[op[i]]; c != 0 {
			upper[i] = c
		} else {
			return strings.ToUpper(string(op)), FlagMayWrite, nil
		}
	}
	op = upper[:len(op)]
	if r, ok := opTable[string(op)]; ok {
		return r.Name, r.Flag, nil
	}
	return string(op), FlagMayWrite, nil
}

func hashSlot(key []byte) int {
	const (
		TagBeg = '{'
		TagEnd = '}'
	)
	if beg := bytes.IndexByte(key, TagBeg); beg >= 0 {
		if end := bytes.IndexByte(key[beg+1:], TagEnd); end >= 0 {
			key = key[beg+1 : beg+1+end]
		}
	}
	return int(crc32.ChecksumIEEE(key) % models.MaxSlotNum)
}

func getHashKey(multi []*redis.Resp, opstr string) []byte {
	var index = 1
	switch opstr {
	case "ZINTERSTORE", "ZUNIONSTORE", "EVAL", "EVALSHA":
		index = 3
	}
	if index < len(multi) {
		return multi[index].Value
	}
	return nil
}
