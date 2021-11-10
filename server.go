package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	iconv "github.com/djimenez/iconv-go"
)
type JsonResult struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

var dataMap map[string]map[string]string


func readConfig() string {
	f, err := os.Open("config")
	if err != nil {
		fmt.Println("read file fail", err)
		return ""
	}
	defer f.Close()

	fd, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Println("read to fd fail", err)
		return ""
	}

	return string(fd)
}

//验证是否在开盘时间内
func checkTime() (res bool) {
	now := time.Now()
	hours, minutes, _ := now.Clock()
	hiStr := strconv.Itoa(hours) + strconv.Itoa(minutes)
	intH1, err := strconv.Atoi(hiStr)
	if err != nil {
		fmt.Println(err)
		return false
	}
	if (intH1 > 900 && intH1 < 1130) || (intH1 > 1300 && intH1 < 1500) {
		return true
	} else {
		fmt.Println("time:", intH1)
		return true
	}
}

//获取数据 到 dataMap
func getStockData(code string) {

	res := checkTime()
	if res == false {
		return
	}

	resp, err := http.Get("https://hq.sinajs.cn/format=text&list=s_" + code)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
	var buffer [512]byte
	result := bytes.NewBuffer(nil)
	for {
		n, err := resp.Body.Read(buffer[0:])
		result.Write(buffer[0:n])
		if err != nil && err == io.EOF {
			break
		} else if err != nil {
			fmt.Println(err)
			return
		}
	}
	tmp := strings.Replace(result.String(), "s_"+code+"=", "", -1)
	data := strings.Split(tmp, ",")
	if len(data) != 6 {
		fmt.Println("代码：", code, "不存在")
		return
	}

	m := make(map[string]string, 3)
	title,_ := iconv.ConvertString(data[0], "gb2312", "utf-8")
	m["title"] = title
	m["zx"] = data[1]
	m["zdf"] = data[3]
	dataMap[code] = m

}

//api服务，实现一个对dataMap的简易分页
func IndexHandle(w http.ResponseWriter, r *http.Request) {

	values := r.URL.Query()
	pageArg := values.Get("page")
	pageSizeArg := values.Get("page_size")
	var page, pageSize int
	if pageArg == "" {
		page = 1
	} else {
		page, _ = strconv.Atoi(pageArg)
	}
	if pageSizeArg == "" {
		pageSize = 10
	} else {
		pageSize, _ = strconv.Atoi(pageSizeArg)
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	i := 0
	tempMap := make(map[string]map[string]string)
	for key, item := range dataMap {
		if i >= start && i < end {
			tempMap[key] = item
		}
		i++
	}
	var countPage = i / pageSize
	var nextPage = 0

	if countPage != page {
		nextPage = page + 1
	}
	JsonData := make(map[string]interface{})
	JsonData["stock"] = tempMap
	JsonData["next_page"] = nextPage
	msg, _ := json.Marshal(JsonResult{Code: 200, Message: "", Data: JsonData})
	w.Header().Set("content-type", "text/json")
	w.Write(msg)
}

func main() {

	dataMap = make(map[string]map[string]string)
	//读取配置
	configStr := readConfig()
	configMap := make(map[int]string)

	configArr := strings.Split(configStr, "\r\n")
	for index, code := range configArr {
		configMap[index] = code
	}

	//每1秒检测一次
	task := make(chan string)
	ticker := time.NewTicker(time.Second * 1)
	go func() {
		for range ticker.C {
			for _, c := range configMap {
				task <- c
			}
		}
	}()

	go func() {
		for {
			select {
			case code := <-task:
				getStockData(code)
			}
		}
	}()

	// 设置路由，如果访问/，则调用index方法
	http.HandleFunc("/", IndexHandle)

	// 启动web服务，监听9090端口
	err := http.ListenAndServe(":9090", nil)
	if err != nil {
		fmt.Println("ListenAndServe: ", err)
		return
	}
}
