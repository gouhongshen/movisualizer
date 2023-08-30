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
	"strconv"
	"time"
	_type "visualization/type"
)

type SpanInfo struct {
	spanSrcDB *gorm.DB
	infos     []_type.SpanInfoTable

	renderData struct {
		Accumulate struct {
			Labels []int
			Data   []int
		}
		TopKDurationAccRate struct {
			Labels []string
			Data   []float32
		}
		TopKDuration struct {
			Labels []string
			Data   []int64
		}
		Heatmap struct {
			Data string
		}
	}
}

var spanInfo *SpanInfo

func VisSpanInfoHandler(w http.ResponseWriter, req *http.Request) {
	if spanInfo == nil {
		spanInfo = new(SpanInfo)
	}

	if _type.SourceFile == "" {
		spanInfo.visualizeByReadDB(w, req)
	} else {
		spanInfo.visBySourceFile(w, req)
	}
}

func (s *SpanInfo) visualizeByReadDB(w http.ResponseWriter, req *http.Request) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/system?charset=utf8mb4&parseTime=True&loc=Local",
		_type.SrcUsrName, _type.SrcPassword, _type.SrcHost, _type.SrcPort)

	if s.spanSrcDB == nil {
		db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			log.Panicf("connect to %s failed", dsn)
		}
		s.spanSrcDB = db
	} else {
		s.infos = make([]_type.SpanInfoTable, 0)
	}

	s.spanSrcDB.Table("span_info").Find(&s.infos)

	s.visualize(w, req)
}

func (s *SpanInfo) visBySourceFile(w http.ResponseWriter, req *http.Request) {

}

func (s *SpanInfo) visualize(w http.ResponseWriter, req *http.Request) {

	wd, _ := os.Getwd()
	tmpl, err := template.ParseFiles(wd + "/script/html/spanInfo.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//var tmplData renderData
	s.visualizeDurAccumulate()
	s.visualizeTopKDuration(30)
	s.visualizeTopKDurationAccumulateRate(10)
	s.visHeatmap()

	if err := tmpl.Execute(w, s.renderData); err != nil {
		fmt.Println(err.Error())
	}
}

func (s *SpanInfo) visHeatmap() {
	//for i := 0; i < 100; i++ {
	//	tmplData.Heatmap.Data[i][0] = i
	//	tmplData.Heatmap.Data[i][1] = i * 2
	//	tmplData.Heatmap.Data[i][2] = rand.Int() % 100
	//}
	s.renderData.Heatmap.Data = "[[0, 0, 5], [1, 0, 12], [2, 0, 20]]"
}

func (s *SpanInfo) visualizeDurAccumulate() {
	data := make(map[int]int)
	for _, info := range s.infos {
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

	s.renderData.Accumulate.Data = freqData
	s.renderData.Accumulate.Labels = labels
}

func (s *SpanInfo) visualizeTopKDuration(topK int) {
	sort.Slice(s.infos, func(i, j int) bool {
		return s.infos[i].Duration > s.infos[j].Duration
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
	for i := 0; i < len(s.infos) && cnt < topK; i++ {
		if _, ok := filter[s.infos[i].SpanName]; ok {
			continue
		}
		labels = append(labels, s.infos[i].NodeType+"->"+s.infos[i].SpanName)
		duration = append(duration, s.infos[i].Duration/int64(time.Millisecond))
		cnt++
	}

	s.renderData.TopKDuration.Data = duration
	s.renderData.TopKDuration.Labels = labels
}

func (s *SpanInfo) visualizeTopKDurationAccumulateRate(topK int) {
	sort.Slice(s.infos, func(i, j int) bool {
		return s.infos[i].SpanName < s.infos[j].SpanName
	})

	data := make([]struct {
		spanName    string
		accDuration int64
		cnt         int
	}, len(s.infos))

	filter := map[string]struct{}{
		"RoutineManager.Handler":             {},
		"batchETLHandler":                    {},
		"QueryStorage.getNewAccounts":        {},
		"QueryStorageStorage.getNewAccounts": {},
	}

	totalDur, idx, i, j := int64(0), 0, 0, 0
	for {
		data[idx].spanName = s.infos[i].SpanName
		for j < len(s.infos) && s.infos[i].SpanName == s.infos[j].SpanName {
			if _, ok := filter[s.infos[j].SpanName]; !ok {
				totalDur += s.infos[j].Duration
			}
			data[idx].accDuration += s.infos[j].Duration
			j++
			data[idx].cnt++
		}

		if j >= len(s.infos) {
			break
		}

		i = j
		idx++
	}

	sort.Slice(data, func(i, j int) bool {
		return data[i].accDuration > data[j].accDuration
	})

	var labels []string
	var rate []float32

	idx = 0
	for i = 0; i < len(data) && idx < topK; i++ {
		if _, ok := filter[data[i].spanName]; ok {
			continue
		}
		labels = append(labels, data[i].spanName+": "+strconv.Itoa(data[i].cnt))
		rate = append(rate, float32(data[i].accDuration)/float32(totalDur))
		idx++
	}

	s.renderData.TopKDurationAccRate.Data = rate
	s.renderData.TopKDurationAccRate.Labels = labels
}
