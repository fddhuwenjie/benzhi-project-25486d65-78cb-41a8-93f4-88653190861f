package model

import (
	"fmt"
	"math"
	"strings"
)

func (b InspectionBatch) Validate() error {
	if strings.TrimSpace(b.ID) == "" || strings.TrimSpace(b.Title) == "" || strings.TrimSpace(b.Collector) == "" {
		return fmt.Errorf("批次标识、标题和责任人不能为空")
	}
	if len(b.ShowcaseIDs) == 0 || !b.WindowEnd.After(b.WindowStart) {
		return fmt.Errorf("展柜范围或采集窗口无效")
	}
	return nil
}

func (o Observation) Validate() error {
	if strings.TrimSpace(o.ID) == "" || strings.TrimSpace(o.BatchID) == "" || strings.TrimSpace(o.ShowcaseID) == "" {
		return fmt.Errorf("观察记录标识不能为空")
	}
	if math.IsNaN(o.TemperatureCelsius) || math.IsNaN(o.RelativeHumidity) || math.IsNaN(o.Lux) {
		return fmt.Errorf("观察读数不能为 NaN")
	}
	return nil
}
