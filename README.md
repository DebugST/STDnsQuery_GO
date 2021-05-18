## STDnsQuery

[![GO1.16](https://img.shields.io/badge/GO-V1.16-blue)](https://golang.google.cn/) [![license](https://img.shields.io/badge/License-MIT-green)](https://github.com/DebugST/STDnsQuery_GO/blob/main/LICENSE) 

STDnsQuery 是一个快速DNS查询工具 其中 DnsClient 是一个方便快捷的调用类 支持 A、NS、CNAME、DNAME 查询 使用简单 

## Install

```
git clone https://github.com/DebugST/STDnsQuery_GO.git
cd STDnsQuery_GO
go build main.go
```

![STDnsQuery](https://raw.githubusercontent.com/DebugST/STDnsQuery_GO/main/Images/Screen%20Shot%202021-05-14%20at%2000.54.29.png)
![STDnsQuery](https://raw.githubusercontent.com/DebugST/STDnsQuery_GO/main/Images/Screen%20Shot%202021-05-14%20at%2000.57.33.png)

GUI(DotNet)[https://github.com/DebugST/STDnsQuery_DotNet](https://github.com/DebugST/STDnsQuery_DotNet)

![STDnsQuery](https://raw.githubusercontent.com/DebugST/STDnsQuery_DotNet/main/Images/Screen%20Shot%202021-05-18%20at%2018.09.00.png)

## Demo

``` go
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
	dns.QueryA("www.github.com", 1, 3)
	dns.QueryFromTask(QueryTask{
		Domain:      "www.github.io",
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
```

## Author
* Github: [DebugST](https://github.com/DebugST/)
* Blog: [Crystal_lz](http://st233.com)
* Mail: (2212233137@qq.com)
