package _type

import (
	"time"
)

var SourceFile string
var DstPort string
var SrcPort string
var SrcHost string
var SrcUsrName string
var SrcPassword string

type SpanInfoTable struct {
	TraceId      string    `gorm:"column:trace_id"`
	SpanId       string    `gorm:"column:span_id"`
	ParentSpanId string    `gorm:"column:span_kind_id"`
	SpanKind     string    `gorm:"column:span_kind"`
	NodeUUID     string    `gorm:"column:node_uuid"`
	NodeType     string    `gorm:"column:node_type"`
	SpanName     string    `gorm:"column:span_name"`
	StartTime    time.Time `gorm:"column:start_time"`
	EndTime      time.Time `gorm:"column:end_time"`
	Duration     int64     `gorm:"column:duration"`
	Resource     string    `gorm:"column:resource"`
	Extra        string    `gorm:"column:extra"`
}

type LogInfoTable struct {
	TraceId    string    `gorm:"column:trace_id"`
	SpanId     string    `gorm:"column:span_id"`
	SpanKind   string    `gorm:"column:span_kind"`
	NodeUuid   string    `gorm:"column:node_uuid"`
	NodeType   string    `gorm:"column:node_type"`
	Timestamp  time.Time `gorm:"column:timestamp"`
	LoggerName string    `gorm:"column:logger_name"`
	Level      string    `gorm:"column:level"`
	Caller     string    `gorm:"column:caller"`
	Message    string    `gorm:"column:message"`
	Extra      string    `gorm:"column:extra"`
	Stack      string    `gorm:"column:stack"`
}
