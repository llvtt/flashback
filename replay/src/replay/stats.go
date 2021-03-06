package replay

import (
	"math/rand"
	"time"
)

// Latency of the mongo ops
type Latency struct {
	OpType  OpType
	Latency time.Duration
}

type IStatsCollector interface {
	StartOp(opType OpType)

	EndOp()

	// How many ops have been captured.
	Count(opType OpType) int64

	// ops/sec for a given op type.
	OpsSec(opType OpType) float64

	// The average latency, which can give you a rough idea of the performance.
	// For fine-grain performance analysis, please enable latency sampling
	// and do the latency analysis by other means.
	LatencyInMs(opType OpType) float64

	// Enable the sampling for latency analysis. Sampled latencies will be sent
	// out via a channel.
	SampleLatencies(sampleRate float64, latencyChannel chan Latency)
}

type StatsCollector struct {
	counts    map[OpType]int64
	durations map[OpType]time.Duration

	total int
	// sample rate will be among [0.0-1.0]
	sampleRate  float64
	epoch       *time.Time
	lastOp      *OpType
	latencyChan chan Latency
}

func NewStatsCollector() *StatsCollector {
	counts := map[OpType]int64{}
	durations := map[OpType]time.Duration{}
	for _, opType := range AllOpTypes {
		counts[opType] = 0
		durations[opType] = 0
	}
	collector := &StatsCollector{
		counts:     counts,
		durations:  durations,
		sampleRate: 1,
	}
	return collector
}

func (s *StatsCollector) StartOp(opType OpType) {
	s.total++
	// should track count of opTypes even if they're not sampled
	s.counts[opType]++

	if s.sampleRate == 0 {
		return
	}

	if s.sampleRate == 1.0 || rand.Float64() < s.sampleRate {
		now := time.Now()
		s.epoch = &now
		s.lastOp = &opType
	}
}

func (s *StatsCollector) EndOp() {
	// This particular op is not sampled
	if s.epoch == nil {
		return
	}

	duration := time.Now().Sub(*s.epoch)
	s.durations[*s.lastOp] += duration
	// s.counts[*s.lastOp]++
	if s.latencyChan != nil {
		s.latencyChan <- Latency{*s.lastOp, duration}
	}
	s.epoch = nil
	s.lastOp = nil
}

func (s *StatsCollector) Count(opType OpType) int64 {
	return s.counts[opType]
}

func (s *StatsCollector) TotalTime(opType OpType) time.Duration {
	return s.durations[opType]
}

func (s *StatsCollector) OpsSec(opType OpType) float64 {
	// TODO: This seems like an unusual way to calculate ops/sec. TotalTime returns the total duration spent doing opType
	// but really we should be dividing total ops / total wall clock time
	// this may explain why ops/sec per-op is much higher than total ops/sec
	nano := s.TotalTime(opType).Nanoseconds()
	if nano == 0 {
		return 0
	}
	return float64(s.counts[opType]) * float64(time.Second) / float64(nano)
}

func (s *StatsCollector) LatencyInMs(opType OpType) float64 {
	count := float64(s.counts[opType])
	if count == 0 {
		return 0
	}
	sec := s.TotalTime(opType).Seconds()
	return sec / count * 1000
}
func (s *StatsCollector) SampleLatencies(sampleRate float64, latencyChannel chan Latency) {
	s.sampleRate = sampleRate
	s.latencyChan = latencyChannel
}

// Combine the stats collected by multiple stats to one.
func CombineStats(statsList ...*StatsCollector) *StatsCollector {
	newStats := NewStatsCollector()

	for _, opType := range AllOpTypes {
		for _, stats := range statsList {
			newStats.counts[opType] += stats.counts[opType]
			newStats.durations[opType] += stats.durations[opType]
			newStats.total += stats.total
		}
	}
	return newStats
}

// NullStatsCollector is a placeholder that does nothing.
type nullStatsCollector struct{}

func (e *nullStatsCollector) StartOp(opType OpType)                                           {}
func (e *nullStatsCollector) EndOp()                                                          {}
func (e *nullStatsCollector) SampleLatencies(sampleRate float64, latencyChannel chan Latency) {}
func (e *nullStatsCollector) Count(opType OpType) int64                                       { return 0 }
func (e *nullStatsCollector) TotalTime(opType OpType) time.Duration                           { return 0 }
func (e *nullStatsCollector) OpsSec(opType OpType) float64                                    { return 0 }
func (e *nullStatsCollector) LatencyInMs(opType OpType) float64                               { return 0 }

// NewNullStatsCollector makes a dumb stats collector that does nothing.
func NewNullStatsCollector() IStatsCollector {
	return &nullStatsCollector{}
}
