package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bndr/gotabulate"

	"github.com/spf13/viper"
)

//定义配置文件解析后的结构
type MySQLConfig struct {
	Name         string
	Email        string
	Passwd       string
	LoginUrl     string
	CheckMethods string
	CheckUrl     string
}

type Config struct {
	ProxyUrl string
	MySQL    []MySQLConfig
}

// 代理client
var clientProxy *http.Client

// 直连client
var clientDirect *http.Client

//定义一个同步等待的组
var wg sync.WaitGroup

// 定义表格数据源
var tableData = make([][]string, 0)

// 生成client
func NewClient(s interface{}) *http.Client {
	var client *http.Client
	jar, _ := cookiejar.New(nil)
	if s == nil {
		client = &http.Client{
			Timeout: time.Second * 10, //超时时间
			Jar:     jar,
		}
		return client
	}
	v, ok := s.(string)
	if ok {
		proxy, _ := url.Parse(v)
		tr := &http.Transport{
			Proxy:           http.ProxyURL(proxy),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{
			Transport: tr,
			Jar:       jar,
			Timeout:   time.Second * 10,
		}
	}
	return client
}

// 读取配置文件
func readInConfig(c *Config) {
	viper.SetConfigName("config") //设置配置文件的名字
	viper.AddConfigPath("./")     //添加配置文件所在的路径
	viper.SetConfigType("json")   //设置配置文件类型，可选
	err := viper.ReadInConfig()   // 读取配置数据
	if err != nil {
		fmt.Printf("config file error: %s\n", err)
		os.Exit(1)
	}
	viper.Unmarshal(c) // 将配置信息绑定到结构体上
}

// 登录
func login(config MySQLConfig) {
	defer wg.Done() //减去一个计数
	form := url.Values{}
	form.Add("email", config.Email)
	form.Add("passwd", config.Passwd)
	form.Add("remember_me", "week")
	req, err := http.NewRequest("POST", config.LoginUrl, strings.NewReader(form.Encode()))
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.130 Safari/537.36")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	n := 0
	resp, err := clientDirect.Do(req)
	if err != nil || config.Name == "CCCAT" {
		req, err := http.NewRequest("POST", config.LoginUrl, strings.NewReader(form.Encode()))
		req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.130 Safari/537.36")
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
		resp, err = clientProxy.Do(req)
		if err != nil {
			tableData = append(tableData, []string{config.Name, "直连登录失败", "代理登录失败"})
			return
		}
		n = 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		tableData = append(tableData, []string{config.Name, "登录失败"})
		return
	}
	body, _ := ioutil.ReadAll(resp.Body)
	priceMap := map[string]interface{}{}
	err = json.Unmarshal(body, &priceMap)
	// Check your errors!
	if err != nil {
		tableData = append(tableData, []string{config.Name, "登录失败"})
		return
	}
	check(config.CheckMethods, config.CheckUrl, config.Name, n)
}

// 签到
func check(methods, url, name string, dorP int) {
	req, err := http.NewRequest(methods, url, nil)
	if err != nil {
		fmt.Println(err)
	}
	var resp *http.Response
	if dorP == 0 {
		resp, err = clientDirect.Do(req)
	} else {
		resp, err = clientProxy.Do(req)
	}
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	defer func() {
		if e := recover(); e != nil {
			tableData = append(tableData, []string{name, "登录成功", "签到失败"})
		}
	}()
	body, _ := ioutil.ReadAll(resp.Body)
	priceMap := map[string]interface{}{}
	err = json.Unmarshal(body, &priceMap)
	// Check your errors!
	if err != nil {
		panic(err)
	}
	value, ok := priceMap["msg"].(string)
	if !ok {
		fmt.Println("failed")
		return
	}
	if dorP == 0 {
		tableData = append(tableData, []string{name, "直连登录成功", value})

	} else {
		tableData = append(tableData, []string{name, "代理登录成功", value})
	}
}

// 按Enter键继续
func pause() {
	fmt.Print("Press 'Enter' to continue...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

func main() {
	var config Config
	readInConfig(&config)
	clientProxy = NewClient(config.ProxyUrl)
	clientDirect = NewClient(nil)
	for i := 0; i < len(config.MySQL); i++ {
		wg.Add(1) //添加一个计数
		go login(config.MySQL[i])
	}
	wg.Wait() //阻塞直到所有任务完成
	// Create Object
	tabulate := gotabulate.Create(tableData)
	// Set Headers
	tabulate.SetHeaders([]string{"Name", "LoginStatus", "CheckStatus"})
	// Render
	fmt.Println(tabulate.Render("simple"))
	pause()
}
