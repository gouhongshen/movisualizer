package script

import (
	"encoding/json"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	_type "visualization/type"
)

type S3Stats struct {
	Labels      []string
	List        []float64
	Head        []float64
	Put         []float64
	Get         []float64
	Delete      []float64
	DeleteMulti []float64
}

type LogInfo struct {
	logSrcDB *gorm.DB
	logs     []_type.LogInfoTable

	renderData struct {
		BlkReadHit struct {
			Labels       []string
			BlkHitRate   []float64
			EntryHitRate []float64
		}
		S3Visit struct {
			FromDN, FromCN S3Stats
		}
		S3VisitDetail struct {
			S3Put struct {
				XLabels   []string
				EntryNums []float64
				DataLens  []float64
			}
			S3Get struct {
				Labels  []string
				GetNums []float64
			}
		}
	}
}

var logInfo *LogInfo

func VisLogInfoHandler(w http.ResponseWriter, req *http.Request) {
	if logInfo == nil {
		logInfo = new(LogInfo)
	}

	if _type.SourceFile == "" {
		logInfo.visualizeLogInfoByReadDB(w, req)
	} else {
		logInfo.visualizeLogInfoBySourceFile(w, req)
	}

	logInfo.visualize(w, req)
}

func (l *LogInfo) visualizeLogInfoByReadDB(w http.ResponseWriter, req *http.Request) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/system?charset=utf8mb4&parseTime=True&loc=Local",
		_type.SrcUsrName, _type.SrcPassword, _type.SrcHost, _type.SrcPort)

	if l.logSrcDB == nil {
		db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Panicf("connect to %s failed", dsn)
		}
		l.logSrcDB = db
	} else {
		l.logs = make([]_type.LogInfoTable, 0)
	}

	l.logSrcDB.Table("log_info").Find(&l.logs)
}

func (l *LogInfo) visualizeLogInfoBySourceFile(w http.ResponseWriter, req *http.Request) {
	file, err := os.Open(_type.SourceFile)
	if err != nil {
		panic(err.Error())
	}

	res, err := io.ReadAll(file)
	data := strings.Split(string(res[:]), "\n\t")
	heads := strings.Split(data[0], "\t")

	for _, str := range data[1:] {
		tt := strings.Split(str, "\t")
		log := _type.LogInfoTable{}
		//typeOf := reflect.TypeOf(log)
		valueOf := reflect.ValueOf(&log).Elem()
		for i := 0; i < len(heads); i++ {
			if heads[i] == "timestamp" {
				log.Timestamp, _ = time.Parse("2023-08-29 10:57:44.349526", tt[i])
			} else {
				heads[i] = strings.ReplaceAll(heads[i], "_", " ")
				heads[i] = strings.Title(heads[i])
				heads[i] = strings.ReplaceAll(heads[i], " ", "")
				v := valueOf.FieldByName(heads[i])
				v.SetString(tt[i])
			}
		}

		l.logs = append(l.logs, log)
	}

}

func (l *LogInfo) visualize(w http.ResponseWriter, req *http.Request) {

	wd, _ := os.Getwd()
	tmpl, err := template.ParseFiles(wd + "/script/html/logInfo.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	l.visualizeBlkReadHitRate()
	l.visualizeS3Visit()
	l.visS3ObjectVisit()

	if err := tmpl.Execute(w, l.renderData); err != nil {
		fmt.Println(err.Error())
	}
}

func (l *LogInfo) visualizeBlkReadHitRate() {
	var blkInfo []_type.LogInfoTable
	for _, log := range l.logs {
		if log.Message == "block read stats" {
			blkInfo = append(blkInfo, log)
		}
	}

	sort.Slice(blkInfo, func(i, j int) bool {
		return blkInfo[i].Timestamp.Before(blkInfo[j].Timestamp)
	})

	stop := false
	for _, info := range blkInfo {
		data := make(map[string]interface{})
		if err := json.Unmarshal([]byte(info.Extra), &data); err != nil {
			fmt.Println(err.Error())
			continue
		}

		rate := data["blk hit rate"].(float64)
		rate2 := data["entry hit rate"].(float64)
		if rate >= 1.0 && data["blk total"].(float64) <= 0.9 {
			if !stop {
				stop = true
				rate, rate2 = 0, 0
			} else {
				continue
			}
		} else {
			stop = false
		}

		l.renderData.BlkReadHit.BlkHitRate = append(l.renderData.BlkReadHit.BlkHitRate, rate)
		l.renderData.BlkReadHit.EntryHitRate = append(l.renderData.BlkReadHit.EntryHitRate, rate2)
		l.renderData.BlkReadHit.Labels = append(l.renderData.BlkReadHit.Labels, info.Timestamp.String())
	}

}

func (l *LogInfo) visualizeS3Visit() {
	var s3Info []_type.LogInfoTable
	for _, log := range l.logs {
		if log.Message == "s3 vis stats" {
			s3Info = append(s3Info, log)
		}
	}

	sort.Slice(s3Info, func(i, j int) bool {
		return s3Info[i].Timestamp.Before(s3Info[j].Timestamp)
	})

	//stop := false
	for _, info := range s3Info {
		data := make(map[string]interface{})
		if err := json.Unmarshal([]byte(info.Extra), &data); err != nil {
			fmt.Println(err.Error())
			continue
		}
		list := data["List"].(float64)
		head := data["Head"].(float64)
		put := data["Put"].(float64)
		get := data["Get"].(float64)
		del := data["Delete"].(float64)
		deleteMulit := data["DeleteMulti"].(float64)

		var from *S3Stats
		if info.NodeType == "DN" {
			from = &l.renderData.S3Visit.FromDN
		} else {
			from = &l.renderData.S3Visit.FromCN
		}

		from.List = append(from.List, list)
		from.Head = append(from.Head, head)
		from.Put = append(from.Put, put)
		from.Get = append(from.Get, get)
		from.Delete = append(from.Delete, del)
		from.DeleteMulti = append(from.DeleteMulti, deleteMulit)
		from.Labels = append(from.Labels, info.Timestamp.String())
	}

}

func (l *LogInfo) visS3ObjectVisit() {
	var s3Info []_type.LogInfoTable
	for _, log := range l.logs {
		if log.Message == "s3 object vis stats" {
			s3Info = append(s3Info, log)
		}
	}

	sort.Slice(s3Info, func(i, j int) bool {
		return s3Info[i].Timestamp.Before(s3Info[j].Timestamp)
	})

	layout := "2006-01-02 15:04:05.000000 -0700 MST"
	start, _ := time.Parse(layout, "2023-08-31 03:11:00.000000 +0800 CST")
	end, _ := time.Parse(layout, "2023-08-31 03:32:00.000000 +0800 CST")

	//s3PutInTPCC := 0
	for _, info := range s3Info {
		if info.NodeType != "CN" {
			continue
		}

		if info.Timestamp.After(end) {
			continue
		}

		data := make(map[string]interface{})
		if err := json.Unmarshal([]byte(info.Extra), &data); err != nil {
			fmt.Println(err.Error())
			continue
		}

		putInfo := strings.Split(data["s3 put stats"].(string), ";")

		for idx := range putInfo {
			str := strings.Split(putInfo[idx], ", ")
			if len(str) < 4 {
				continue
			}
			// name, cnt, entry num, byte size
			byteSize, _ := strconv.ParseFloat(str[3], 32)
			entryNum, _ := strconv.ParseFloat(str[2], 32)
			l.renderData.S3VisitDetail.S3Put.DataLens = append(l.renderData.S3VisitDetail.S3Put.DataLens, byteSize)
			l.renderData.S3VisitDetail.S3Put.EntryNums = append(l.renderData.S3VisitDetail.S3Put.EntryNums, entryNum)
			l.renderData.S3VisitDetail.S3Put.XLabels = append(l.renderData.S3VisitDetail.S3Put.XLabels, info.Timestamp.String())

			//if info.Timestamp.After(time.P)
		}
	}
	// 0~1K, 1K ~ 10K, 10K ~ 100K, 100K ~ 1000K
	records := make(map[int]int)
	total := len(l.renderData.S3VisitDetail.S3Put.DataLens)
	for _, val := range l.renderData.S3VisitDetail.S3Put.DataLens {
		x := int(val / 1024.0)
		if x == 0 {
			records[0]++
		} else if x >= 1 && x < 10 {
			records[1]++
		} else if x >= 10 && x < 100 {
			records[2]++
		} else if x >= 100 && x < 1000 {
			records[3]++
		} else {
			records[4]++
		}

	}

	for idx, v := range records {
		str := []string{
			"0K~1K", "1K~10K", "10K~100K", "100K~1000K", ">=1M",
		}
		fmt.Printf("%s: %.2f%s\n", str[idx], float32(v)/float32(total)*100, "%")
	}

	tmp := make(map[string]float64)
	for _, info := range s3Info {
		if info.NodeType != "CN" {
			continue
		}

		if info.Timestamp.After(end) || info.Timestamp.Before(start) {
			continue
		}

		data := make(map[string]interface{})
		if err := json.Unmarshal([]byte(info.Extra), &data); err != nil {
			fmt.Println(err.Error())
			continue
		}

		getInfo := strings.Split(data["s3 get stats"].(string), ";")

		for idx := range getInfo {
			str := strings.Split(getInfo[idx], ", ")
			if len(str) < 3 {
				continue
			}
			//name, cnt, rowCnt
			getCnt, _ := strconv.ParseFloat(str[1], 32)
			tmp[str[0]] += getCnt
		}
	}

	for idx, _ := range tmp {
		if strings.HasSuffix(idx, ".csv") {
			continue
		}

		l.renderData.S3VisitDetail.S3Get.GetNums = append(l.renderData.S3VisitDetail.S3Get.GetNums, tmp[idx])
		l.renderData.S3VisitDetail.S3Get.Labels = append(l.renderData.S3VisitDetail.S3Get.Labels, idx)
	}
}
