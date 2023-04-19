package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

type StageInfo struct {
	NIngestSynConnections   int
	NIngestSynBatchLen      int
	NIngestSynLogsPerMin    int
	NTransformGenericRules  int
	NTransformNetworkRules  int
	NTransformFilterRules   int
	NConnTrackOutputFields  int
	NExtractAggregateRules  int
	NEncodePromMetricsRules int
	StageTypeNames          []string
}

type MetricsStruct struct {
	TimeFromStart float64
	Cpu           float64
	Memory        float64
	NFlows        float64
	NProm         float64
}

func CreateTargetFile(fileName string) (*os.File, error) {
	dir := filepath.Dir(fileName)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		fmt.Printf("error creating output directory; err = %v \n", err)
		return nil, err
	}
	f, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	return f, err
}
