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
