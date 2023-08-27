package _type

import "time"

type SpanInfo struct {
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
