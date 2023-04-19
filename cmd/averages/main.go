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
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type Options struct {
	srcFolder   string
	tgtFolder   string
	discardTime time.Duration
}

var opts Options

const (
	defaultTgtDir       = "/tmp/averages"
	defaultSrcDir       = "/tmp/perfmeasurements"
	perfResultsExt      = ".csv"
	defaultdiscardTime  = 5 * time.Minute
	resultsFolderPrefix = "averages_"
)

// rootCmd represents the root command
var rootCmd = &cobra.Command{
	Use:   "averages",
	Short: "Collect averages from perf measurments files",
	Run: func(cmd *cobra.Command, args []string) {
		run()
	},
}

func initFlags() {
	rootCmd.PersistentFlags().StringVar(&opts.srcFolder, "srcFolder", defaultSrcDir, "source folder")
	rootCmd.PersistentFlags().StringVar(&opts.tgtFolder, "tgtFolder", defaultTgtDir, "target folder")
	rootCmd.PersistentFlags().DurationVar(&opts.discardTime, "discardTime", defaultdiscardTime, "amount of data to discard at beginning of run")
}

func printFlags() {
	fmt.Printf("srcFolder = %s \n", opts.srcFolder)
	fmt.Printf("tgtFolder = %s \n", opts.tgtFolder)
	fmt.Printf("discardTime = %v \n", opts.discardTime)
}

func printFilePaths(filePaths []string) {
	fmt.Printf("filepaths of data files: \n")
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
	dirPaths := getDirPaths(opts.srcFolder, "")
	fmt.Printf("dirPaths = %v \n", dirPaths)
	tgtFolder := opts.tgtFolder + "/" + resultsFolderPrefix + time.Now().Format(time.RFC3339)
	err := createTargetFolder(tgtFolder)
	if err != nil {
		fmt.Printf("could not create target folder; err = %v, dirName = %s \n", err, opts.tgtFolder)
		os.Exit(1)
	}
	calculateAverages(opts.srcFolder, dirPaths, tgtFolder)
}

func getDirPaths(rootPath string, prefix string) []string {
	var directories []string

	newRootPath := filepath.Join(rootPath, prefix)
	dirEntries, err := os.ReadDir(newRootPath)
	if err != nil {
		fmt.Printf("could not read directory; err = %v, dirName = %s \n", err, rootPath)
		return nil
	}
	for _, f := range dirEntries {
		fMode := f.Type()
		fName := f.Name()
		if fMode.IsDir() {
			fPath := filepath.Join(prefix, fName)
			subDirs := getDirPaths(rootPath, fPath)
			if len(subDirs) > 0 {
				directories = append(directories, subDirs...)
			} else {
				directories = append(directories, fPath)
			}
		}
	}
	return directories
}

func getCsvFileNames(rootPath string, prefix string) []string {
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
		if fMode.IsRegular() && filepath.Ext(fName) == perfResultsExt {
			if err != nil {
				fmt.Printf("could not obtain file path name; err = %v, fileName = %s \n", err, f.Name())
				return nil
			}
			fileName := filepath.Join(prefix, fName)
			files = append(files, fileName)
		}
		if fMode.IsDir() {
			fPath := filepath.Join(prefix, fName)
			subDirFiles := getCsvFileNames(rootPath, fPath)
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

func calculateFileAverages(records [][]string, dw *bufio.Writer) {
	var metrics utils.MetricsStruct
	var si utils.StageInfo
	// first line should contain column headers; ignore it
	record := records[1]
	si.NIngestSynConnections, _ = strconv.Atoi((record[5]))
	si.NIngestSynBatchLen, _ = strconv.Atoi((record[6]))
	si.NIngestSynLogsPerMin, _ = strconv.Atoi((record[7]))
	si.NTransformFilterRules, _ = strconv.Atoi((record[8]))
	si.NTransformGenericRules, _ = strconv.Atoi((record[9]))
	si.NTransformNetworkRules, _ = strconv.Atoi((record[10]))
	si.NConnTrackOutputFields, _ = strconv.Atoi((record[11]))
	si.NExtractAggregateRules, _ = strconv.Atoi((record[12]))
	si.NEncodePromMetricsRules, _ = strconv.Atoi((record[13]))
	fmt.Printf("stage type names = %v \n", record[14])
	tmpString := record[14][1 : len(record[14])-1]
	si.StageTypeNames = strings.Split(tmpString, " ")
	fmt.Printf("stage type names = %v \n", si.StageTypeNames)
	nItems := 0
	memSum := 0.0
	memMax := 0.0
	promSum := 0.0
	promMax := 0.0
	for _, record := range records[1:] {
		metrics.TimeFromStart, _ = strconv.ParseFloat(record[0], 64)
		if metrics.TimeFromStart < float64(opts.discardTime.Seconds()) {
			// discard this entry since it is during the warm up period
			continue
		}
		metrics.Cpu, _ = strconv.ParseFloat(record[1], 64)
		metrics.Memory, _ = strconv.ParseFloat(record[2], 64)
		metrics.NFlows, _ = strconv.ParseFloat(record[3], 64)
		metrics.NProm, _ = strconv.ParseFloat(record[4], 64)
		memSum += metrics.Memory
		if metrics.Memory > memMax {
			memMax = metrics.Memory
		}
		promSum += metrics.NProm
		if metrics.NProm > promMax {
			promMax = metrics.NProm
		}
		nItems++
	}
	if nItems > 0 {
		memAvg := memSum / float64(nItems)
		promAvg := promSum / float64(nItems)
		cpuRate := metrics.Cpu / metrics.TimeFromStart
		l := fmt.Sprintf("%f,%f,%f,%f,%f,%d,%d,%d,%d,%d,%d,%d,%d,%d,%s\n",
			cpuRate, memAvg, memMax, promAvg, promMax,
			si.NIngestSynConnections, si.NIngestSynBatchLen, si.NIngestSynLogsPerMin,
			si.NTransformFilterRules, si.NTransformGenericRules, si.NTransformNetworkRules,
			si.NConnTrackOutputFields, si.NExtractAggregateRules, si.NEncodePromMetricsRules,
			si.StageTypeNames,
		)
		_, _ = dw.WriteString(l)
		dw.Flush()
	}
}

func calculateAverages(srcFolder string, dirPaths []string, tgtFolder string) {
	for _, dir := range dirPaths {
		srcDirPath := srcFolder + dir
		tgtFile := tgtFolder + "/" + dir + ".csv"
		fmt.Printf("srcDirPath = %v \n", srcDirPath)
		fmt.Printf("tgtFile = %v \n", tgtFile)

		targetF, err := utils.CreateTargetFile(tgtFile)
		if err != nil {
			fmt.Println("Error: ", err)
			continue
		}
		dw := bufio.NewWriter(targetF)
		// write the csv column headers
		l := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
			"Cpu Rate", "Memory Avg", "Memory Max", "nPromMetrics Avg", "nPromMetrics Max",
			"# connections", "batch len", "flow logs per min",
			"transform filter rules", "transform generic rules", "transform network rules",
			"conntrack output fields", "aggregates rules", "prom metrics rules",
			"stage type names",
		)
		_, _ = dw.WriteString(l)
		dw.Flush()

		filePaths := getCsvFileNames(srcDirPath, "")
		//printFilePaths(filePaths)
		for _, fPath := range filePaths {
			fullFilePath := filepath.Join(srcDirPath, fPath)
			fmt.Printf("fullFilePath = %s \n", fullFilePath)
			f, err := os.Open(fullFilePath)
			if err != nil {
				log.Fatal(err)
				continue
			}

			// read csv values using csv.Reader
			csvReader := csv.NewReader(f)
			records, err := csvReader.ReadAll()
			f.Close()
			if err != nil {
				log.Fatal(err)
				continue
			}
			calculateFileAverages(records, dw)
		}
		targetF.Close()
	}
}
