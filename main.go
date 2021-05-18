package main

import (
	. "STLib/DnsQuery"
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	g_true    int
	g_counter int
	g_runed   int
	g_timeout int
	g_out     *os.File
	g_lock    sync.Mutex
)

func main() {
	var (
		a_int_thread  int
		a_int_retries int
		a_int_timeout int
		a_str_domain  string
		a_str_servers string
		a_str_dicfile string
		a_str_outfile string
	)
	flag.IntVar(&a_int_thread, "c", 1000, "[Concurrent] Set the number of concurrency.")
	flag.IntVar(&a_int_retries, "r", 1, "[Retries] The retries when query timeout.")
	flag.IntVar(&a_int_timeout, "t", 3, "[Timeout] Set the timeout")
	flag.StringVar(&a_str_domain, "d", "", "[Domain] The domian that want to query")
	flag.StringVar(&a_str_servers, "s", "8.8.8.8:53,8.8.4.4:53", "[Servers] Specify the servers to be used.")
	flag.StringVar(&a_str_outfile, "out", "./result.json", "[OutFile] Specify the file to be output.")
	flag.StringVar(&a_str_dicfile, "dic", "./dic.txt", "[DicFile] Specify the domain prefix file.")
	flag.Parse()
	a_str_domain = strings.Trim(a_str_domain, ".")
	//a_str_domain = ".baidu.com"
	if a_str_domain == "" {
		fmt.Println("[Error] - Invalid domain. Please use -h to show help")
		return
	}
	dns := NewDnsClient(uint16(a_int_thread), DefaultCallBack)
	nConcurrent, err := dns.Start(strings.Split(a_str_servers, ","))
	if err != nil {
		fmt.Println("[Error] - Start:", err)
		return
	}
	fmt.Println("[+OK]   - Started:", nConcurrent)
	dicFile, err := os.Open(a_str_dicfile)
	if err != nil {
		fmt.Println("[Error] - OpenFile:", a_str_dicfile, "|", err)
		return
	}
	defer dicFile.Close()
	g_out, err = os.Create(a_str_outfile)
	if err != nil {
		fmt.Println("[Error] - CreateFile:", a_str_outfile, "|", err)
		return
	}
	defer g_out.Close()

	time_start := time.Now()

	go startQuery(dicFile, a_int_retries, a_int_timeout, a_str_domain, dns)

	i := 0
	for {
		i++
		time.Sleep(1 * time.Second)
		fmt.Println(i, "Started:", g_counter, " Runed:", g_runed, " Results:", g_true, " Timeout:", g_timeout)
		if g_counter == g_runed {
			break
		}
	}
	if g_true != 0 {
		g_out.WriteString("]")
	}

	time_end := time.Now()
	fmt.Println("Time - Start:", time_start.Format("2006-01-02 15:04:05"))
	fmt.Println("Time - End:  ", time_end.Format("2006-01-02 15:04:05"))
	fmt.Println("Time - Sub:  ", time_end.Sub(time_start))
}

func startQuery(file *os.File, retries int, timeout int, strRootDomain string, dnsQuery *DnsClient) {
	file.Seek(0, io.SeekStart)
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	strRootDomain = "." + strings.Trim(strRootDomain, ".")
	qt := QueryTask{
		Retries: retries,
		Timeout: timeout,
	}
	for scanner.Scan() {
		qt.Domain = scanner.Text() + strRootDomain
		_, err := dnsQuery.QueryFromTask(qt)
		if err == nil {
			g_lock.Lock()
			g_counter++
			g_lock.Unlock()
		}
	}
}

func DefaultCallBack(result QueryResult, bFromCache bool) {
	g_lock.Lock()
	switch result.RCode {
	case RCODE_NONE:
		byBuffer, _ := json.Marshal(result)
		if g_true == 0 {
			g_out.WriteString(fmt.Sprintf("[\r\n%s", byBuffer))
		} else {
			g_out.WriteString(fmt.Sprintf(",%s\r\n", byBuffer))
		}
		g_true++
	case RCODE_TIMEOUT:
		g_timeout++
	}
	g_runed++
	g_lock.Unlock()
}
