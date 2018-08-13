package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type LogEntry struct {
	Timestamp     float64 `json:"ts"`
	Message       string  `json:"msg"`
	VcapRequestID string  `json:"vcap-request-id"`
	RequestID     int64   `json:"request-id"`
}

type TimestampAndEntry struct {
	timestamp float64
	value     LogEntry
}

func main() {
	filename := os.Args[1]
	file, e := os.Open(filename)
	if e != nil {
		panic(e)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 100*1024*1024)
	scanner.Buffer(buf, 100*1024*1024)
	logEntries := make(map[int64]LogEntry)
	for scanner.Scan() {
		var entry LogEntry
		e := json.Unmarshal(scanner.Bytes(), &entry)
		if e != nil {
			continue
		}
		if strings.Contains(entry.Message, "HTTP Request started") {
			logEntries[entry.RequestID] = entry
		}
		if strings.Contains(entry.Message, "HTTP Request completed") {
			delete(logEntries, entry.RequestID)
		}

		if _, exists := logEntries[entry.RequestID]; exists {
			logEntries[entry.RequestID] = entry
		}
	}
	sortedEntries := make([]TimestampAndEntry, 0)
	for _, request := range logEntries {
		sortedEntries = append(sortedEntries, TimestampAndEntry{request.Timestamp, request})
	}
	sort.Slice(sortedEntries, func(i int, j int) bool { return sortedEntries[i].timestamp < sortedEntries[j].timestamp })

	for _, entry := range sortedEntries {
		fmt.Printf("%v: (id: %v, vcap-request-id: %v): %v\n",
			time.Unix(int64(entry.value.Timestamp), 0),
			entry.value.RequestID,
			entry.value.VcapRequestID,
			entry.value.Message)
	}
}
