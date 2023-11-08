package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

type Day struct {
	Name     string `json:"name"`
	Date     string `json:"date"`
	IsOffDay bool   `json:"isOffDay"`
}

type Result struct {
	Day     string `json:"day"`
	Holiday bool   `json:"holiday"`
	Weekday string `json:"weekday"`
}

func init() {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		fmt.Println(err)
		return
	}
	time.Local = loc
}

var tYear *string

func main() {
	tYear = flag.String("year", "", "请输入需要处理的年份")
	flag.Parse()
	if len(*tYear) < 1 {
		fmt.Println("年份为空")
		return
	}
	url := "https://raw.githubusercontent.com/NateScarlet/holiday-cn/master/" + *tYear + ".json"
	fmt.Println(url)

	// 发送HTTP GET请求获取数据
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("请求失败:", err)
		return
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("读取响应失败:", err)
		return
	}
	fmt.Println(string(body))

	// 解析JSON数据
	var data struct {
		Days []Day `json:"days"`
	}
	err = json.Unmarshal(body, &data)
	if err != nil {
		fmt.Println("解析JSON失败:", err)
		return
	}

	// 预处理日期信息，构建日期到Day的映射
	dayMap := make(map[string]Day)
	for _, day := range data.Days {
		dayMap[day.Date] = day
	}

	// 生成当前年份的所有日期
	year, err := strconv.Atoi(*tYear)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	startDate := time.Date(year, time.January, 1, 0, 0, 0, 0, time.Local)
	endDate := time.Date(year, time.December, 31, 0, 0, 0, 0, time.Local)
	daysCount := int(endDate.Sub(startDate).Hours() / 24)
	allDates := make([]time.Time, daysCount+1)
	for i := 0; i <= daysCount; i++ {
		allDates[i] = startDate.AddDate(0, 0, i)
	}

	// 限制并发数量为最多10个
	concurrencyLimit := 10
	semaphore := make(chan struct{}, concurrencyLimit)
	// 并发处理日期信息
	var wg sync.WaitGroup

	// 结果存储
	type resultData struct {
		Days     []Result `json:"days"`
		Holidays []string `json:"holidays"`
		Workdays []string `json:"workdays"`
	}
	rd := resultData{
		Days: make([]Result, len(allDates)),
	}

	// 启动并发任务处理日期信息
	for i, date := range allDates {
		wg.Add(1)
		semaphore <- struct{}{} // 获取信号量，限制并发数量
		go func(index int, d time.Time) {
			defer func() {
				<-semaphore // 释放信号量
				wg.Done()
			}()

			// 格式化日期
			dateStr := d.Format("2006-01-02")

			// 查找日期对应的信息
			isHoliday := false
			weekday := d.Weekday().String()
			if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
				isHoliday = true
			}

			if day, ok := dayMap[dateStr]; ok {
				isHoliday = day.IsOffDay
			}
			if isHoliday {
				rd.Holidays = append(rd.Holidays, dateStr)
			} else {
				rd.Workdays = append(rd.Workdays, dateStr)
			}

			rs := Result{
				Day:     dateStr,
				Holiday: isHoliday,
				Weekday: weekday,
			}
			rd.Days[index] = rs
		}(i, date)
	}

	// 等待所有并发任务完成
	wg.Wait()

	sort.Strings(rd.Holidays)
	sort.Strings(rd.Workdays)

	// 将结果转换为JSON格式
	resultJSON, err := json.Marshal(rd)
	if err != nil {
		fmt.Println("转换结果为JSON失败:", err)
		return
	}

	// 将结果写入文件
	err = os.WriteFile("./data/"+(*tYear)+".json", resultJSON, 0644)
	if err != nil {
		fmt.Println("写入文件失败:", err)
		return
	}

	hods := map[string][]string{}
	hods["holidays"] = rd.Holidays

	// 将结果转换为JSON格式
	rhoJSON, _ := json.Marshal(hods)

	// 将结果写入文件
	err = os.WriteFile("./data/"+(*tYear)+"-holidays.json", rhoJSON, 0644)
	if err != nil {
		fmt.Println("写入文件失败:", err)
		return
	}

	fmt.Println("结果已存储在文件中")
}
