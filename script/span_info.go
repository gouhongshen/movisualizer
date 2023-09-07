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

var PageData struct {
	ObjVisFrequency struct {
		Labels []string
		Data   []float64
	}
	DataSizeTotal struct {
		Labels []string
		Data   []float64
	}
	FrequencyByDuration struct {
		Labels []string
		Data   []int64
	}
	DurationDistribution struct {
		Labels []string
		Data   []int64
	}
}

type SpanInfo struct {
	spanSrcDB *gorm.DB
	//infos     []_type.SpanInfoTable
	//
	//renderData struct {
	//	LocalFSOperation, S3FSOperation struct {
	//		ObjVisFrequency struct {
	//			Labels []string
	//			Data   []float64
	//		}
	//		DataSizeTotal struct {
	//			Labels []string
	//			Data   []float64
	//		}
	//		FrequencyByDuration struct {
	//			Labels []int
	//			Data   []int64
	//		}
	//		DurationDistribution struct {
	//			Labels []string
	//			Data   []int64
	//		}
	//	}
	//}
}

const (
	S3FSOperation    int = 0
	LocalFSOperation int = 1
)

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
		//s.infos = make([]_type.SpanInfoTable, 0)
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

	//s.visLocalFSOperation()
	s.visS3FSOperation()

	if err := tmpl.Execute(w, PageData); err != nil {
		fmt.Println(err.Error())
	}
}

func (s *SpanInfo) visS3FSOperation() {
	var infos []_type.SpanInfoTable
	s.spanSrcDB.Table("span_info").Where("span_kind='s3FSOperation'").Find(&infos)

	s.visObjVisFrequency(infos, S3FSOperation)
	s.visObjDataSize(infos, S3FSOperation)
	s.visDurationFrequency(infos, S3FSOperation)
	s.visDurationDistribution(infos, S3FSOperation)

}

func (s *SpanInfo) visLocalFSOperation() {
	var infos []_type.SpanInfoTable
	s.spanSrcDB.Table("span_info").Where("span_kind='localFSOperation'").Find(&infos)

	s.visObjVisFrequency(infos, LocalFSOperation)
	s.visObjDataSize(infos, LocalFSOperation)
	s.visDurationFrequency(infos, LocalFSOperation)
}

func (s *SpanInfo) visDurationFrequency(infos []_type.SpanInfoTable, t int) {
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].EndTime.Before(infos[j].EndTime)
	})

	var condition string
	if t == S3FSOperation {
		condition = "S3FS.read"
	} else {
		condition = "LocalFS.read"
	}

	var cntByDuration []int64
	var endTime []string
	//sec := 0
	last := infos[0].EndTime

	idx := 0
	for idx < len(infos) {
		cnt := int64(0)
		for idx < len(infos) && infos[idx].EndTime.Sub(last) <= time.Second {
			if infos[idx].SpanName == condition {
				cnt++
			}
			idx++
		}

		if idx < len(infos) {
			last = infos[idx].EndTime
		}

		endTime = append(endTime, infos[idx-1].EndTime.String())
		cntByDuration = append(cntByDuration, cnt)
	}

	for idx, cnt := range cntByDuration {
		PageData.FrequencyByDuration.Labels = append(PageData.FrequencyByDuration.Labels, endTime[idx])
		PageData.FrequencyByDuration.Data = append(PageData.FrequencyByDuration.Data, cnt)
	}

}

func (s *SpanInfo) visObjDataSize(infos []_type.SpanInfoTable, t int) {
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
		PageData.DataSizeTotal.Labels = append(PageData.DataSizeTotal.Labels, objName[idx])
		PageData.DataSizeTotal.Data = append(PageData.DataSizeTotal.Data, name2Size[objName[idx]])
	}

}

func (s *SpanInfo) visObjVisFrequency(infos []_type.SpanInfoTable, t int) {
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
		PageData.ObjVisFrequency.Labels = append(PageData.ObjVisFrequency.Labels, objName[idx])
		PageData.ObjVisFrequency.Data = append(PageData.ObjVisFrequency.Data, name2Cnt[objName[idx]])
	}

	fmt.Println(cnt)
}

func (s *SpanInfo) visDurationDistribution(infos []_type.SpanInfoTable, t int) {
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].EndTime.Before(infos[j].EndTime)
	})

	var condition string
	if t == S3FSOperation {
		condition = "S3FS.read"
	} else {
		condition = "LocalFS.read"
	}

	for idx, _ := range infos {
		if infos[idx].SpanName != condition {
			continue
		}

		PageData.DurationDistribution.Labels = append(PageData.DurationDistribution.Labels, infos[idx].EndTime.String())
		PageData.DurationDistribution.Data = append(PageData.DurationDistribution.Data, infos[idx].Duration)
	}

}
