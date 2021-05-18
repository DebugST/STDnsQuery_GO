package DnsQuery

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type DnsClient struct {
	runed           int
	maxTask         uint16
	queID           chan uint16
	queIdleTask     chan *queryTaskInner
	queWaitTask     chan *queryTaskInner
	dicRunning      map[uint16]*queryTaskInner
	lock            sync.Mutex
	callBack        func(result QueryResult, bFromCache bool)
	dnsUDPAddr      []*net.UDPAddr
	dnsAddr         map[string]byte
	dnsServersCount int
	started         bool
	closed          bool
}

func (d *DnsClient) GetRuned() int      { return d.runed }
func (d *DnsClient) GetMaxTask() int    { return int(d.maxTask) }
func (d *DnsClient) GetRunning() int    { return len(d.dicRunning) }
func (d *DnsClient) GetIsStarted() bool { return d.started }

func init() {
	g_map_cache = make(map[string]cacheData)
	go func() {
		// := make([]string, 1000)
		for {
			time.Sleep(time.Second)
			g_time_now = time.Now().Unix()
			g_map_cache_lock.Lock()
			for k, v := range g_map_cache {
				if g_time_now > v.exprieTime {
					//keysTemp = append(keysTemp, k)
					delete(g_map_cache, k)
				}
			}
			// for _, k := range keysTemp {
			// 	delete(g_map_cache, k)
			// }
			g_map_cache_lock.Unlock()
		}
	}()
}

func (qt *queryTaskInner) initFromQueryTask(t QueryTask, uID uint16, buffer []byte) {
	qt.domain = t.Domain
	qt.queryType = t.Type
	qt.retries = t.Retries
	qt.timeout = t.Timeout
	qt.callBack = t.CallBack

	qt.retried = 0
	qt.sended = false

	qt.dnsAddr = nil
	if t.DnsServer != "" {
		qt.dnsAddr, _ = net.ResolveUDPAddr("udp", t.DnsServer)
	}
	if qt.timeout == 0 {
		qt.timeout = 3
	}

	qt.id = uID
	buffer[0] = byte(uID >> 8)
	buffer[1] = byte(uID)
	qt.queryPackage = buffer
}

func NewDnsClient(maxTask uint16, callBack func(result QueryResult, bFromCache bool)) *DnsClient {
	ret := &DnsClient{
		maxTask:     maxTask,
		queID:       make(chan uint16, 0xFFFF-1),
		queIdleTask: make(chan *queryTaskInner, maxTask),
		queWaitTask: make(chan *queryTaskInner, maxTask),
		dicRunning:  make(map[uint16]*queryTaskInner),
		dnsAddr:     make(map[string]byte),
		callBack:    callBack,
	}
	if maxTask == 0xFFFF {
		maxTask = 0xFFFE
	}
	for i := 1; i < 0xFFFF; i++ {
		ret.queID <- uint16(i)
	}
	for i := 1; i < int(maxTask+1); i++ {
		ret.queIdleTask <- &queryTaskInner{}
	}
	return ret
}

func (d *DnsClient) setDnsServers(strServers []string) error {
	d.started = true
	servers := make([]*net.UDPAddr, len(strServers))
	for i, str := range strServers {
		if _, ok := d.dnsAddr[str]; ok {
			continue
		}
		s, e := net.ResolveUDPAddr("udp", str)
		if e != nil {
			return e
		}
		d.dnsAddr[str] = 0
		servers[i] = s
	}
	d.dnsUDPAddr = servers
	d.dnsServersCount = len(servers)
	return nil
}

func (d *DnsClient) Start(dnsServers []string) (int, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.started {
		return 0, errors.New("dnsClient has started")
	}
	ret := 0
	if err := d.setDnsServers(dnsServers); err != nil {
		return 0, err
	}
	d.started = true
	for i := 0; i < int(d.maxTask); i++ {
		sock, err := net.ListenUDP("udp", &net.UDPAddr{
			IP:   net.IPv4(0, 0, 0, 0),
			Port: 0,
		})
		if err == nil {
			ret++
		}
		go d.startRecv(sock)
		go d.startSend(sock)
	}
	go d.checkTimeout()
	return ret, nil
}

func (d *DnsClient) startSend(sock *net.UDPConn) {
	for !d.closed {
		task, err := <-d.queWaitTask
		if !err {
			return
		}
		addr := task.dnsAddr
		if addr == nil {
			addr = d.dnsUDPAddr[(task.id+uint16(task.retried))%uint16(d.dnsServersCount)]
		}
		sock.WriteToUDP(task.queryPackage, addr)
		//if error will resend by CheckTimeout() until [Retried] == [Retries]
		task.lastTime = time.Now()
		task.sended = true
	}
	sock.Close()
}

func (d *DnsClient) startRecv(sock *net.UDPConn) {
	byBuffer := make([]byte, 1440)
	for !d.closed {
		nLen, addr, err := sock.ReadFrom(byBuffer)
		if nLen == 0 || err != nil {
			break
		}
		if nLen < 21 {
			continue
		}
		strAddr := addr.String()
		if _, ok := d.dnsAddr[strAddr]; !ok {
			fmt.Println(strAddr)
			continue
		}
		result, err := GetDnsResponse(byBuffer)
		if err != nil {
			continue
		}
		result.DnsServer = strAddr
		d.endTask(result.ID, result, true)
		CacheResult(result)
	}
}

func (d *DnsClient) addDic(id uint16, task *queryTaskInner) {
	d.lock.Lock()
	d.dicRunning[id] = task
	d.lock.Unlock()
}

func (d *DnsClient) removeDic(id uint16, bSync bool) *queryTaskInner {
	if bSync {
		d.lock.Lock()
	}
	task, ok := d.dicRunning[id]
	if ok {
		delete(d.dicRunning, id)
		d.runed++
	}
	if bSync {
		d.lock.Unlock()
	}
	return task
}

func (d *DnsClient) endTask(id uint16, result QueryResult, bSync bool) {
	task := d.removeDic(id, bSync)
	if task == nil {
		//fmt.Println(id, "removed")
		return
	}
	task.callBack(result, false)
	if d.closed {
		return
	}
	d.queID <- task.id
	d.queIdleTask <- task
}

func (d *DnsClient) checkTimeout() {
	for !d.closed {
		time.Sleep(1 * time.Second)
		ti := time.Now()
		d.lock.Lock()
		for _, t := range d.dicRunning {
			if int(ti.Sub(t.lastTime).Seconds()) < t.timeout || !t.sended {
				continue
			}
			t.sended = false
			if t.retried == t.retries {
				d.endTask(t.id, QueryResult{
					RCode:  RCODE_TIMEOUT,
					Type:   t.queryType,
					Domain: t.domain,
				}, false)
				continue
			}
			t.retried++
			t.dnsAddr = d.dnsUDPAddr[(int(t.id)+t.retried)%d.dnsServersCount]
			d.queWaitTask <- t
		}
		d.lock.Unlock()
	}
}

func (d *DnsClient) QueryA(strDomain string, retries int, timeout int) (uint16, error) {
	qt := QueryTask{
		Type:    TYPE_A,
		Domain:  strDomain,
		Retries: retries,
		Timeout: timeout,
	}
	return d.QueryFromTask(qt)
}

func (d *DnsClient) QueryFromTask(task QueryTask) (nID uint16, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("dnsClient was closed\r\n%v", err)
		}
	}()
	if d.closed {
		return 0, errors.New("dnsClent was closed")
	}
	if task.CallBack == nil {
		task.CallBack = d.callBack
	}
	if task.Type == 0 {
		task.Type = TYPE_A
	}
	if !task.IgnoreCache {
		if result, ok := GetCachedResult(task.Type, task.Domain); ok {
			go task.CallBack(result, true)
			return 0, nil
		}
	}
	byBuffer, err := GetDnsRequest(task.Type, task.Domain)
	if err != nil {
		return 0, err
	}
	t, ok_task := <-d.queIdleTask
	nID, ok_id := <-d.queID
	if !(ok_task && ok_id) {
		return 0, errors.New("dnsClent was closed")
	}
	t.initFromQueryTask(task, nID, byBuffer)
	t.queryPackage = byBuffer
	if t.dnsAddr == nil {
		t.dnsAddr = d.dnsUDPAddr[int(t.id)%d.dnsServersCount]
	}

	d.queWaitTask <- t
	d.addDic(t.id, t)

	return t.id, nil
}

func (d *DnsClient) Close() {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.closed {
		return
	}
	d.closed = true
	close(d.queID)
	close(d.queIdleTask)
	close(d.queWaitTask)
	d.dicRunning = make(map[uint16]*queryTaskInner)
}

//========================================================================================================

func GetDnsRequest(queryType Type, strDomain string) ([]byte, error) {
	buffer := []byte{
		0x00, 0x00, //ID
		0x01, 0x00, //Standard query
		0x00, 0x01, //questions :1
		0x00, 0x00, //answer rrs :0
		0x00, 0x00, //authority rrs:0
		0x00, 0x00, //additional rrs:0
	}
	strs := strings.Split(strDomain, ".")
	for _, s := range strs {
		if len(s) > 0xFF || len(s) == 0 {
			return nil, errors.New("ivalid domain")
		}
		buffer = append(buffer, byte(len(s)))
		buffer = append(buffer, []byte(s)...)
	}
	buffer = append(buffer, []byte{
		0x00,                  //Domain End
		0x00, byte(queryType), //QueryType
		0x00, byte(CLASS_IN)}..., //Class
	)
	return buffer, nil
}

func GetDnsResponse(byBuffer []byte) (result QueryResult, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("an not format the package to dns response\r\n%v", e)
		}
	}()
	//The dnsResponse minsize is 21. [4][5] = questions = 1
	if len(byBuffer) < 21 || byBuffer[4] != 0 || byBuffer[5] != 1 {
		return result, errors.New("can not format the package to dns response")
	}
	//1 000 0... .0.. 0000 , response:1bit opcode:4bit resvered:1bit rcode:4bit
	if byBuffer[3]&0xF8 != 0x80 || byBuffer[3]&0x40 != 0 || byBuffer[3]&0xF > 5 {
		return result, errors.New("can not format the package to dns response")
	}
	result.ID = (uint16(byBuffer[0]) << 8) + uint16(byBuffer[1])
	result.RCode = RCode(byBuffer[3] & 0x0F)
	nIndex := 12
	nIndex, result.Domain = getDomainFromBuffer(byBuffer, g_str_domain_buffer, nIndex, 0)
	result.Type = Type(byBuffer[nIndex+1])
	if result.RCode != 0 {
		return result, nil
	}
	nIndex += 4
	result.Answers = getAnswers(byBuffer, nIndex)
	return result, nil
}

func getAnswers(byBuffer []byte, nIndex int) []QueryAnswer {
	answers := make([]QueryAnswer, int(byBuffer[7])+(int(byBuffer[6])<<8))
	for i := 0; i < len(answers); i++ {
		bBreak := false
		nIndex, answers[i].Domain = getDomainFromBuffer(byBuffer, g_str_domain_buffer, nIndex, 0)
		answers[i].Type = Type(int(byBuffer[nIndex+1]) + (int(byBuffer[nIndex]) << 8))
		nIndex += 2
		answers[i].Class = Class(int(byBuffer[nIndex+1]) + (int(byBuffer[nIndex]) << 8))
		nIndex += 2
		answers[i].TTL = int(byBuffer[nIndex+3]) + (int(byBuffer[nIndex+2]) << 8) + (int(byBuffer[nIndex+1]) << 16) + (int(byBuffer[nIndex]) << 24)
		nIndex += 4
		switch answers[i].Type {
		case TYPE_A:
			nIndex += 2
			answers[i].Data = fmt.Sprintf("%d.%d.%d.%d", byBuffer[nIndex], byBuffer[nIndex+1], byBuffer[nIndex+2], byBuffer[nIndex+3])
			nIndex += 4
		case TYPE_NS:
			fallthrough
		case TYPE_DNAME:
			fallthrough
		case TYPE_CNAME:
			nIndex += 2
			nIndex, answers[i].Data = getDomainFromBuffer(byBuffer, g_str_domain_buffer, nIndex, int(byBuffer[nIndex-1])+(int(byBuffer[nIndex-2])<<8))
		default:
			if i == 0 {
				answers = nil
			} else {
				answers_new := make([]QueryAnswer, i)
				copy(answers_new, answers)
			}
			bBreak = true
		}
		if bBreak {
			break
		}
	}
	return answers
}

func getDomainFromBuffer(byBuffer []byte, strBuffer []string, nIndex int, nDataLen int) (int, string) {
	g_str_domain_buffer_lock.Lock()
	strBuffer = strBuffer[0:0]
	nOldIndex := 0

	nMaxIndex := nIndex + nDataLen
	for byBuffer[nIndex] != 0 {
		if nDataLen != 0 && nIndex >= nMaxIndex {
			break
		}
		if (byBuffer[nIndex] & 0xC0) == 0xC0 {
			if nOldIndex == 0 {
				nOldIndex = nIndex
			}
			nIndex = int(byBuffer[nIndex+1]) + (int(int(byBuffer[nIndex])&(int(^0xC0))) << 8)
		}
		str := string(byBuffer[nIndex+1 : nIndex+1+int(byBuffer[nIndex])])
		strBuffer = append(strBuffer, str)
		nIndex = nIndex + int(byBuffer[nIndex]) + 1
	}
	nIndex++
	if nOldIndex != 0 {
		nIndex = (nOldIndex + 2)
	}
	domain := strings.Join(strBuffer, ".")
	g_str_domain_buffer_lock.Unlock()
	return nIndex, domain
}

func GetCachedResult(queryType Type, strDomain string) (QueryResult, bool) {
	strKey := strings.ToLower(fmt.Sprintf("%d|%s", queryType, strDomain))
	g_map_cache_lock.RLock()
	cache, ok := g_map_cache[strKey]
	g_map_cache_lock.RUnlock()
	if !ok {
		return QueryResult{}, false
	}
	if g_time_now > cache.exprieTime {
		g_map_cache_lock.Lock()
		delete(g_map_cache, strKey)
		g_map_cache_lock.Unlock()
		return QueryResult{}, false
	}
	return cache.queryResult, true
}

func CacheResult(result QueryResult) {
	if len(result.Answers) == 0 {
		return
	}
	ts := time.Now()
	strKey := strings.ToLower(fmt.Sprintf("%d|%s", result.Type, result.Domain))
	te := ts.Add(time.Duration(result.Answers[0].TTL * int(time.Second)))
	result.ID = 0
	data := cacheData{
		exprieTime:  te.Unix(),
		queryResult: result,
	}
	g_map_cache_lock.Lock()
	g_map_cache[strKey] = data
	for _, r := range result.Answers {
		strKey := fmt.Sprintf("%d|%s", r.Type, r.Domain)
		te := ts.Add(time.Duration(r.TTL * int(time.Second)))
		data := cacheData{
			exprieTime:  te.Unix(),
			queryResult: result,
		}
		g_map_cache[strKey] = data
	}
	g_map_cache_lock.Unlock()
}

func ClearCache() {
	g_map_cache_lock.Lock()
	g_map_cache = make(map[string]cacheData)
	g_map_cache_lock.Unlock()
}
