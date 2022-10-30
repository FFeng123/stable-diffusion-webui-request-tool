package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
)

const configFile = "config.json"
const loginString = "username=%s&password=%s"
const reloginErrn = 3 // 多少次网络错误后重新登录

var client = &http.Client{}

type ConfigStruct struct {
	Host     string `json:"host"`
	Usr      string `json:"usr"`
	Pwd      string `json:"pwd"`
	DrawArgs string `json:"args"`
	SaveFile string `json:"save"`
	Seed     int64  `json:"seed"`
	Minsize  int    `json:"minsize"`
	session  string
}

func log(logtype string, logtext string, isln bool) {
	if isln {
		fmt.Printf("[%s]%s\n", logtype, logtext)
	} else {
		fmt.Printf("[%s]%s", logtype, logtext)
	}
}

var config ConfigStruct

func main() {
	if !readConfig() {
		return
	}

	// session放了东西之后会失败，不放反而成功
	//rand.Seed(time.Now().UnixNano())
	//config.session = fmt.Sprint(rand.Int())
	netErrn := 4
	for {
		if netErrn > reloginErrn {
			if !login() {
				return
			}
		}
		if !getImage(config.Seed) {
			netErrn += 1
			if netErrn > reloginErrn+1 {
				log("错误", "能够登录但是无法发送请求，请检测参数", true)
				return
			}
		} else {
			netErrn = 0
		}
		config.Seed += 1
	}
}

func readConfig() bool {
	bt, err1 := ioutil.ReadFile(configFile)
	if err1 != nil {
		log("错误", "读取配置文件时出错："+err1.Error(), true)
		return false
	}
	err1 = json.Unmarshal(bt, &config)
	if err1 != nil {
		log("错误", "解析配置文件时出错："+err1.Error(), true)
		return false
	}

	if config.Host[len(config.Host)-1] != '/' {
		config.Host = config.Host + "/"
	}

	return true
}

func login() bool {
	log("信息", "正在登录到服务器", true)
	jar, err0 := cookiejar.New(nil)

	if err0 != nil {
		log("错误", "登录失败："+err0.Error(), true)
		return false
	}

	client.Jar = jar

	re, err := client.Post(config.Host+"login", "application/x-www-form-urlencoded", bytes.NewBuffer([]byte(fmt.Sprintf(loginString, config.Usr, config.Pwd))))
	if err != nil {
		log("错误", "登录失败："+err.Error(), true)
		return false
	}
	if re.StatusCode != 200 {
		log("错误", "登录失败："+fmt.Sprint(re.StatusCode), true)
		return false
	}
	defer re.Body.Close()

	log("信息", "登录成功", true)
	return true
}

// 返回网络连接是否正常
func getImage(seed int64) bool {
	log("信息", "请求服务器AI绘制"+fmt.Sprint(seed), true)
	re, err1 := client.Post(config.Host+"api/predict/", "application/json", bytes.NewBuffer([]byte(fmt.Sprintf("{\"fn_index\":14,\"data\":%s,\"session_hash\":%s\"\"}", fmt.Sprintf(config.DrawArgs, fmt.Sprint(seed)), config.session))))

	if err1 != nil {
		log("错误", "请求失败："+err1.Error(), true)
		return false
	}
	defer re.Body.Close()
	bt, err2 := ioutil.ReadAll(re.Body)
	log("信息", fmt.Sprintf("得到长度为%d的回应数据", len(bt)), true)
	if err2 != nil {
		log("错误", "请求失败："+err2.Error(), true)
		return false
	}
	restr := string(bt)

	namest := strings.Index(restr, "\"name\":\"/")
	if namest == -1 {
		log("错误", "请求失败：服务器未返回文件名(1)", true)
		return false
	}
	namest += 8
	nameed := strings.Index(restr[namest:], "\"")
	if nameed == -1 {
		log("错误", "请求失败：服务器未返回文件名(2)", true)
		return false
	}
	// 获取文件
	urls := config.Host + "file=" + restr[namest:nameed+namest]
	log("信息", "得到图片地址，开始请求图片文件", true)

	re2, err3 := client.Get(urls)
	if err3 != nil {
		log("错误", "文件请求失败："+err3.Error(), true)
		return false
	}
	defer re2.Body.Close()
	bt, err4 := ioutil.ReadAll(re2.Body)
	if err4 != nil {
		log("错误", "文件请求失败："+err4.Error(), true)
		return false
	}

	log("信息", fmt.Sprintf("得到长度为%d的图片数据", len(bt)), true)
	if len(bt) < config.Minsize {
		log("信息", "图片信息量不足，已舍弃", true)
		return true
	}

	log("信息", "开始写入文件", true)
	err5 := ioutil.WriteFile(fmt.Sprintf(config.SaveFile, fmt.Sprint(seed)), bt, fs.FileMode(os.O_WRONLY))

	if err5 != nil {
		log("错误", "写文件失败："+err5.Error(), true)
		return true
	}
	return true
}
