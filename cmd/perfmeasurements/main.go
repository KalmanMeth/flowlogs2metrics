/*
 * Copyright (C) 2023 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sys/unix"
)

type Options struct {
	srcFolder               string
	tgtFolder               string
	timeBetweenMeasurements time.Duration
	timeToRun               time.Duration
}

var opts Options

const (
	defaultSrcDir       = "contrib/measurements"
	defaultTgtDir       = "/tmp/perfmeasurements"
	defaultTick         = 2 * time.Second
	defaultTimeToRun    = 20 * time.Second
	definitionExt       = ".yaml"
	resultExt           = ".csv"
	flpExec             = "flowlogs-pipeline"
	resultsFolderPrefix = "perf_"
)

// rootCmd represents the root command
var rootCmd = &cobra.Command{
	Use:   "perfmeasurements",
	Short: "Run performance measurements on specified config files",
	Run: func(cmd *cobra.Command, args []string) {
		run()
	},
}

func initFlags() {
	rootCmd.PersistentFlags().StringVar(&opts.srcFolder, "srcFolder", defaultSrcDir, "source folder")
	rootCmd.PersistentFlags().StringVar(&opts.tgtFolder, "tgtFolder", defaultTgtDir, "target folder")
	rootCmd.PersistentFlags().DurationVar(&opts.timeBetweenMeasurements, "timeBetweenMeasurements", defaultTick, "time between measurements")
	rootCmd.PersistentFlags().DurationVar(&opts.timeToRun, "timeToRun", defaultTimeToRun, "time to run each test")
}

func printFlags() {
	fmt.Printf("srcFolder = %s \n", opts.srcFolder)
	fmt.Printf("tgtFolder = %s \n", opts.tgtFolder)
	fmt.Printf("timeBetweenMeasurements = %v \n", opts.timeBetweenMeasurements)
	fmt.Printf("timeToRun = %v \n", opts.timeToRun)
}

func printFilePaths(filePaths []string) {
	fmt.Printf("filepaths of configuration files: \n")
	for _, f := range filePaths {
		fmt.Printf("%s \n", f)
	}
}

func main() {
	// Initialize flags (command line parameters)
	initFlags()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() {
	// Dump the configuration
	printFlags()
	filePaths := getYamlFileNames(opts.srcFolder, "")
	printFilePaths(filePaths)
	tgtFolder := opts.tgtFolder + "/" + resultsFolderPrefix + time.Now().Format(time.RFC3339)
	err := createTargetFolder(tgtFolder)
	if err != nil {
		fmt.Printf("could not create target folder; err = %v, dirName = %s \n", err, opts.tgtFolder)
		os.Exit(1)
	}
	runMeasurements(opts.srcFolder, filePaths, tgtFolder)
}

func getYamlFileNames(rootPath string, prefix string) []string {
	var files []string

	newRootPath := filepath.Join(rootPath, prefix)
	dirEntries, err := os.ReadDir(newRootPath)
	if err != nil {
		fmt.Printf("could not read directory; err = %v, dirName = %s \n", err, rootPath)
		return nil
	}
	for _, f := range dirEntries {
		fMode := f.Type()
		fName := f.Name()
		if fMode.IsRegular() && filepath.Ext(fName) == definitionExt {
			if err != nil {
				fmt.Printf("could not obtain file path name; err = %v, fileName = %s \n", err, f.Name())
				return nil
			}
			fileName := filepath.Join(prefix, fName)
			files = append(files, fileName)
		}
		if fMode.IsDir() {
			fPath := filepath.Join(prefix, fName)
			subDirFiles := getYamlFileNames(rootPath, fPath)
			files = append(files, subDirFiles...)
		}
	}
	return files
}

func createTargetFolder(folderName string) error {
	err := os.MkdirAll(folderName, 0755)
	if err != nil {
		log.Debugf("os.MkdirAll err: %v ", err)
		return err
	}
	return nil
}

func runMeasurements(srcFolder string, filePaths []string, tgtFolder string) {
	cwd, _ := os.Getwd()
	flp := cwd + "/" + flpExec
	for _, fPath := range filePaths {
		fmt.Printf("running measurements on %s \n", fPath)
		fullFilePath := filepath.Join(srcFolder, fPath)
		fmt.Printf("fullFilePath = %s \n", fullFilePath)
		configStruct := readConfig(fullFilePath)
		if configStruct == nil {
			fmt.Printf("error in reading config file: %s \n", fullFilePath)
			continue
		}
		si := extractStageInfo(configStruct)
		fmt.Printf("StageInfo = %v \n", si)
		cmd := exec.Command(flp, "--config", fullFilePath)
		if err := cmd.Start(); err != nil {
			fmt.Println("Error: ", err)
			continue
		}

		startTime := time.Now()
		fmt.Printf("start time = %s \n", startTime.Format(time.RFC3339))
		ticker := time.NewTicker(opts.timeBetweenMeasurements)
		done := make(chan bool)

		// create results file
		fileName := filepath.Join(tgtFolder, fPath)
		// change the file extension
		fileName = fileName[:len(fileName)-len(filepath.Ext(fileName))] + resultExt
		fmt.Printf("output file name = %s \n", fileName)
		f, err := utils.CreateTargetFile(fileName)
		if err != nil {
			fmt.Println("Error: ", err)
			continue
		}
		dw := bufio.NewWriter(f)
		// write the csv column headers
		l := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
			"TimeFromStart", "Cpu", "Memory", "NFlows", "nPromMetrics",
			"# connections", "batch len", "flow logs per min",
			"transform filter rules", "transform generic rules", "transform network rules",
			"conntrack output fields", "aggregates rules", "prom metrics rules",
			"stage type names",
		)
		_, _ = dw.WriteString(l)
		dw.Flush()

		go func() {
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					metrics, err := collectMetrics()
					if err != nil {
						continue
					}
					currentTime := time.Now()
					timeFromStart := currentTime.Sub(startTime)
					metrics.TimeFromStart = timeFromStart.Seconds()
					l := fmt.Sprintf("%f,%f,%f,%f,%f,%d,%d,%d,%d,%d,%d,%d,%d,%d,%s\n",
						metrics.TimeFromStart, metrics.Cpu, metrics.Memory, metrics.NFlows, metrics.NProm,
						si.NIngestSynConnections, si.NIngestSynBatchLen, si.NIngestSynLogsPerMin,
						si.NTransformFilterRules, si.NTransformGenericRules, si.NTransformNetworkRules,
						si.NConnTrackOutputFields, si.NExtractAggregateRules, si.NEncodePromMetricsRules,
						si.StageTypeNames,
					)
					_, _ = dw.WriteString(l)
					dw.Flush()
				}
			}
		}()

		go func() {
			time.Sleep(opts.timeToRun)
			ticker.Stop()
			done <- true

			// kill the flp process
			_ = cmd.Process.Signal(unix.SIGINT)
		}()

		_ = cmd.Wait()
		_ = f.Close()
	}
}

func collectMetrics() (utils.MetricsStruct, error) {
	resp, err := http.Get("http://localhost:9102/metrics")
	if err != nil {
		fmt.Println("Error: ", err)
		return utils.MetricsStruct{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error: ", err)
	}
	lines := strings.Split(string(body), "\n")

	var cpu float64
	var memory float64
	var nFlows float64
	var nProm float64
	for _, line := range lines {
		if strings.HasPrefix(line, "flp_op_ingest_synthetic_flows_processed") {
			s := strings.Split(line, " ")
			nFlows, err = strconv.ParseFloat(s[1], 64)
			if err != nil {
				fmt.Printf("error converting NFlows; s = %v, err = %v \n", s, err)
				continue
			}
		}
		if strings.HasPrefix(line, "flp_op_encode_prom_metrics_reported") {
			s := strings.Split(line, " ")
			nProm, err = strconv.ParseFloat(s[1], 64)
			if err != nil {
				fmt.Printf("error converting NProm; s = %v, err = %v \n", s, err)
				continue
			}
		}
		if strings.HasPrefix(line, "process_cpu_seconds_total") {
			s := strings.Split(line, " ")
			cpu, err = strconv.ParseFloat(s[1], 64)
			if err != nil {
				fmt.Printf("error converting Cpu; s = %v, err = %v \n", s, err)
				continue
			}
		}
		if strings.HasPrefix(line, "process_resident_memory_bytes") {
			s := strings.Split(line, " ")
			memory, err = strconv.ParseFloat(s[1], 64)
			if err != nil {
				fmt.Printf("error converting Memory; s = %v, err = %v \n", s, err)
				continue
			}
		}
	}
	metrics := utils.MetricsStruct{
		Cpu:    cpu,
		Memory: memory,
		NFlows: nFlows,
		NProm:  nProm,
	}
	return metrics, nil
}

func readConfig(confFileName string) *config.ConfigFileStruct {
	yamlConfig, _ := os.ReadFile(confFileName)
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	v := viper.New()
	v.SetConfigType("yaml")
	r := bytes.NewReader(yamlConfig)
	_ = v.ReadConfig(r)

	var b []byte
	pipelineStr := v.Get("pipeline")
	b, _ = json.Marshal(&pipelineStr)
	options := config.Options{}
	options.PipeLine = string(b)

	parametersStr := v.Get("parameters")
	b, _ = json.Marshal(&parametersStr)
	options.Parameters = string(b)

	metricsSettingsStr := v.Get("metricsSettings")
	b, _ = json.Marshal(&metricsSettingsStr)
	options.MetricsSettings = string(b)

	out, _ := config.ParseConfig(options)

	return &out
}

func extractStageInfo(configInfo *config.ConfigFileStruct) *utils.StageInfo {
	si := utils.StageInfo{}
	var stageTypes []string
	for _, p := range configInfo.Parameters {
		var stageType string
		if p.Ingest != nil {
			if p.Ingest.Type == "synthetic" {
				si.NIngestSynConnections += p.Ingest.Synthetic.Connections
				si.NIngestSynBatchLen += p.Ingest.Synthetic.BatchMaxLen
				si.NIngestSynLogsPerMin += p.Ingest.Synthetic.FlowLogsPerMin
			}
			stageType = "ingest_" + p.Ingest.Type
		}
		if p.Transform != nil {
			stageType = "transform_" + p.Transform.Type
			switch p.Transform.Type {
			case "filter":
				si.NTransformFilterRules += len(p.Transform.Filter.Rules)
			case "generic":
				si.NTransformGenericRules += len(p.Transform.Generic.Rules)
			case "network":
				si.NTransformNetworkRules += len(p.Transform.Network.Rules)
			}
		}
		if p.Extract != nil {
			stageType = "extract_" + p.Extract.Type
			switch p.Extract.Type {
			case "aggregates":
				si.NExtractAggregateRules += len(p.Extract.Aggregates)
			case "conntrack":
				si.NConnTrackOutputFields += len(p.Extract.ConnTrack.OutputFields)
			}
		}
		if p.Encode != nil {
			stageType = "encode_" + p.Encode.Type
			switch p.Encode.Type {
			case "prom":
				si.NEncodePromMetricsRules += len(p.Encode.Prom.Metrics)
			}
		}
		stageTypes = append(stageTypes, stageType)
	}
	si.StageTypeNames = stageTypes
	return &si
}
