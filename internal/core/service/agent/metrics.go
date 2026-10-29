package agent

import "time"

var RecordAnswerMetric = func(float64, bool) {}
var RecordToolMetric = func(string, string) {}
var RecordRunMetric = func(string, time.Duration, int, int) {}
var RecordCanaryMetric = func(string) {}
