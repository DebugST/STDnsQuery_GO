package DnsQuery

import (
	"net"
	"sync"
	"time"
)

var g_str_domain_buffer_lock sync.Mutex
var g_str_domain_buffer []string = make([]string, 0)

var g_map_cache_lock sync.RWMutex
var g_map_cache map[string]cacheData

var g_time_now int64

type Type uint16
type Class uint16
type RCode byte

const (
	TYPE_A     Type = 0x01
	TYPE_NS    Type = 0x02
	TYPE_CNAME Type = 0x05
	TYPE_DNAME Type = 0x27
)

const (
	CLASS_IN     Class = 0x01
	CLASS_CSNET  Class = 0x02
	CLASS_CHAOS  Class = 0x03
	CLASS_HESIOD Class = 0x04
	CLASS_ANY    Class = 0xFF
)

const (
	RCODE_NONE          RCode = 0x00
	RCODE_FORMATERROR   RCode = 0x01
	RCODE_SERVERFAILURE RCode = 0x02
	RCODE_NOTFOUND      RCode = 0x03
	RCODE_UNKNOWTYPE    RCode = 0x04
	RCODE_REJECT        RCode = 0x05
	RCODE_TIMEOUT       RCode = 0xFF
)

type QueryTask struct {
	Domain      string `json:"doamin"`
	Type        Type   `json:"type"`
	DnsServer   string `json:"dnsServer"`
	Timeout     int    `json:"timeout"`
	Retries     int    `json:"retries"`
	IgnoreCache bool   `json:"ignoreCache"`
	CallBack    func(result QueryResult, bFromCache bool)
}

type QueryAnswer struct {
	Domain string `json:"domain"`
	Type   Type   `json:"type"`
	Class  Class  `json:"class"`
	TTL    int    `json:"ttl"`
	Data   string `json:"data"`
}

type QueryResult struct {
	ID        uint16        `json:"-"`
	RCode     RCode         `json:"rcode"`
	Domain    string        `json:"domain"`
	Type      Type          `json:"type"`
	DnsServer string        `json:"dnsServer"`
	Answers   []QueryAnswer `json:"answers"`
}

type queryTaskInner struct {
	id           uint16
	domain       string
	queryType    Type
	dnsAddr      *net.UDPAddr
	retries      int
	retried      int
	timeout      int
	lastTime     time.Time
	sended       bool
	queryPackage []byte
	callBack     func(QueryResult, bool)
}

type cacheData struct {
	exprieTime  int64
	queryResult QueryResult
}
