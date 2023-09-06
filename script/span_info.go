package script

import (
	"encoding/json"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"time"
	_type "visualization/type"
)

type SpanInfo struct {
	spanSrcDB *gorm.DB
	infos     []_type.SpanInfoTable

	renderData struct {
		LocalFSOperation struct {
			ObjVisFrequency struct {
				Labels []string
				Data   []float64
			}
			DataSizeTotal struct {
				Labels []string
				Data   []float64
			}
			FrequencyByDuration struct {
				Labels []int
				Data   []int64
			}
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

	s.visLocalFSOperation()

	if err := tmpl.Execute(w, s.renderData); err != nil {
		fmt.Println(err.Error())
	}
}

func (s *SpanInfo) visLocalFSOperation() {
	var infos []_type.SpanInfoTable
	s.spanSrcDB.Table("span_info").Where("span_kind='localFSOperation'").Find(&infos)

	s.visLocalFSOperation_ObjVisFrequency(infos)
	s.visLocalFSOperation_ObjDataSize(infos)
	s.visLocalFSOperation_DurationFrequency(infos)
}

func (s *SpanInfo) visLocalFSOperation_DurationFrequency(infos []_type.SpanInfoTable) {
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].EndTime.Before(infos[j].EndTime)
	})

	cntByDuration := make([]int64, 1)
	sec := 0
	last := infos[0].EndTime
	for idx, _ := range infos {
		if infos[idx].EndTime.Sub(last) <= time.Second {
			cntByDuration[sec]++
		} else {
			cntByDuration = append(cntByDuration, 0)
			sec++
			last = infos[idx].EndTime
			cntByDuration[sec]++
		}
	}

	for idx, cnt := range cntByDuration {
		s.renderData.LocalFSOperation.FrequencyByDuration.Labels =
			append(s.renderData.LocalFSOperation.FrequencyByDuration.Labels, idx)
		s.renderData.LocalFSOperation.FrequencyByDuration.Data =
			append(s.renderData.LocalFSOperation.FrequencyByDuration.Data, cnt)
	}

}

func (s *SpanInfo) visLocalFSOperation_ObjDataSize(infos []_type.SpanInfoTable) {
	var objName []string
	name2Size := make(map[string]float64)

	for idx, _ := range infos {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(infos[idx].Extra), &data); err != nil {
			fmt.Println(fmt.Errorf("json unmarsh extra failed"))
		}

		if len(data) == 0 {
			continue
		}

		name := data["name"].(string)
		size := data["size"].(float64)
		_, ok := name2Size[name]
		// each read bytes could be different for a same object
		name2Size[name] += size

		if !ok {
			objName = append(objName, data["name"].(string))
		}
	}

	sort.Slice(objName, func(i, j int) bool {
		return name2Size[objName[i]] > name2Size[objName[j]]
	})

	for idx, _ := range objName {
		s.renderData.LocalFSOperation.DataSizeTotal.Labels =
			append(s.renderData.LocalFSOperation.DataSizeTotal.Labels, objName[idx])
		s.renderData.LocalFSOperation.DataSizeTotal.Data =
			append(s.renderData.LocalFSOperation.DataSizeTotal.Data, name2Size[objName[idx]])
	}

}

func (s *SpanInfo) visLocalFSOperation_ObjVisFrequency(infos []_type.SpanInfoTable) {
	var objName []string
	name2Cnt := make(map[string]float64)

	cnt := 0
	for idx, _ := range infos {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(infos[idx].Extra), &data); err != nil {
			fmt.Println(fmt.Errorf("json unmarsh extra failed"))
		}

		if len(data) == 0 {
			continue
		}

		name := data["name"].(string)
		_, ok := name2Cnt[name]
		name2Cnt[name]++

		if !ok {
			objName = append(objName, data["name"].(string))
		}
		cnt++
	}

	sort.Slice(objName, func(i, j int) bool {
		return name2Cnt[objName[i]] > name2Cnt[objName[j]]
	})

	for idx, _ := range objName {
		s.renderData.LocalFSOperation.ObjVisFrequency.Labels =
			append(s.renderData.LocalFSOperation.ObjVisFrequency.Labels, objName[idx])
		s.renderData.LocalFSOperation.ObjVisFrequency.Data =
			append(s.renderData.LocalFSOperation.ObjVisFrequency.Data, name2Cnt[objName[idx]])
	}

	fmt.Println(cnt)
}
