package utils

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
