package script

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"time"
	_type "visualization/type"
)

type renderData struct {
	Accumulate struct {
		Labels []int
		Data   []int
	}
	TopKDuration struct {
		Labels []string
		Data   []int64
	}
}

var SpanSrcDB *gorm.DB
var infos []_type.SpanInfo

func VisSpanInfoHandler(w http.ResponseWriter, req *http.Request) {
	if _type.SourceFile == "" {
		visualizeByReadDB(w, req)
	} else {
		visBySourceFile(w, req)
	}
}

func visualizeByReadDB(w http.ResponseWriter, req *http.Request) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/system?charset=utf8mb4&parseTime=True&loc=Local",
		_type.SrcUsrName, _type.SrcPassword, _type.SrcHost, _type.SrcPort)

	if SpanSrcDB == nil {
		db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Panicf("connect to %s failed", dsn)
		}
		SpanSrcDB = db
	} else {
		infos = make([]_type.SpanInfo, 0)
	}

	SpanSrcDB.Table("span_info").Find(&infos)

	visualize(w, req, infos)
}

func visBySourceFile(w http.ResponseWriter, req *http.Request) {

}

func visualize(w http.ResponseWriter, req *http.Request, infos []_type.SpanInfo) {

	wd, _ := os.Getwd()
	tmpl, err := template.ParseFiles(wd + "/script/html/spanInfo.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var tmplData renderData
	visualizeDurAccumulate(&tmplData, infos)
	visualizeTopKDuration(&tmplData, 10, infos)

	if err := tmpl.Execute(w, tmplData); err != nil {
		fmt.Println(err.Error())
	}
}

func visualizeDurAccumulate(tmplData *renderData, infos []_type.SpanInfo) {
	data := make(map[int]int)
	for _, info := range infos {
		cur := info.Duration / int64(time.Millisecond)
		data[int(math.Ceil(float64(cur)/100.0)*100)]++
	}

	labels := make([]int, 0)
	freqData := make([]int, 0)
	cum := 0
	keys := []int{0}
	for key, _ := range data {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	for _, key := range keys {
		labels = append(labels, key)
		freqData = append(freqData, data[key]+cum)
		cum += data[key]
	}

	tmplData.Accumulate.Data = freqData
	tmplData.Accumulate.Labels = labels
}

func visualizeTopKDuration(tmplData *renderData, topK int, infos []_type.SpanInfo) {
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Duration > infos[j].Duration
	})

	var labels []string
	var duration []int64

	filter := map[string]struct{}{
		"RoutineManager.Handler":             {},
		"batchETLHandler":                    {},
		"QueryStorage.getNewAccounts":        {},
		"QueryStorageStorage.getNewAccounts": {},
	}

	cnt := 0
	for i := 0; i < len(infos) && cnt < topK; i++ {
		if _, ok := filter[infos[i].SpanName]; ok {
			continue
		}
		labels = append(labels, infos[i].NodeType+"->"+infos[i].SpanName)
		duration = append(duration, infos[i].Duration/int64(time.Millisecond))
		cnt++
	}

	tmplData.TopKDuration.Data = duration
	tmplData.TopKDuration.Labels = labels
}
