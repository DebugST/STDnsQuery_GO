package main

import (
	. "STLib/DnsQuery"
	"encoding/json"
	"fmt"
)

func Test() {
	dns := NewDnsClient(1000, defaultCallBack)
	dns.Start([]string{
		"8.8.8.8:53",
		"8.8.4.4:53",
	})
	dns.QueryA("www.google.com", 1, 3)
	dns.QueryFromTask(QueryTask{
		Domain:      "api.google.com",
		Type:        TYPE_A,       //默认TYPE_A
		Timeout:     5,            //超时时间 默认3
		Retries:     3,            //重试次数 默认1
		IgnoreCache: true,         //若当前有缓存数据 是否忽略
		DnsServer:   "1.1.1.1:53", //若指定服务器 则忽略dns.Start([]string)传入的服务器
		CallBack: func(result QueryResult, bFromCache bool) {
			fmt.Println(result.Domain, result.RCode)
		}, //若指定回调函数 则defaultCallBack不会触发
	})
}

func defaultCallBack(result QueryResult, bFromCache bool) {
	if result.RCode == RCODE_NONE {
		byBuffer, _ := json.Marshal(result)
		fmt.Println(string(byBuffer))
	}
}
