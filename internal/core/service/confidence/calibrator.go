package confidence

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// PartitionCalibration holds per-kb_type quality score distribution stats.
type PartitionCalibration struct {
	KBType      string    `json:"kb_type"`
	P20         float64   `json:"p20"` // top 20% threshold
	P60         float64   `json:"p60"` // top 60% threshold
	SampleCount int       `json:"sample_count"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CalibrationRepo provides historical quality score distributions.
type CalibrationRepo interface {
	// GetQualityPercentiles returns P20 and P60 thresholds for a kb_type,
	// along with sample count. Returns (0,0,0) if insufficient data.
	GetQualityPercentiles(ctx context.Context, kbType string) (p20, p60 float64, sampleCount int, err error)
}

// Calibrator maps raw quality scores to percentile-based grades per partition.
// Eliminates hardcoded thresholds (0.72, 0.50) by learning from actual data.
type Calibrator struct {
	mu      sync.RWMutex
	byKB    map[string]*PartitionCalibration
	repo    CalibrationRepo
	refresh time.Duration
	// Fallback thresholds used when partition data is insufficient (SampleCount < 100)
	FallbackP20 float64
	FallbackP60 float64
}

// NewCalibrator creates a calibrator with periodic background refresh.
func NewCalibrator(repo CalibrationRepo, refreshInterval time.Duration) *Calibrator {
	c := &Calibrator{
		byKB:        make(map[string]*PartitionCalibration),
		repo:        repo,
		refresh:     refreshInterval,
		FallbackP20: 0.72,
		FallbackP60: 0.50,
	}
	if c.refresh <= 0 {
		c.refresh = 6 * time.Hour
	}
	return c
}

// Start begins periodic background refresh.
func (c *Calibrator) Start(ctx context.Context) {
	go c.refreshLoop(ctx)
}

// CalibrateGrade maps a raw quality score to a grade for the given kb_type.
// Falls back to global thresholds when partition data is insufficient.
func (c *Calibrator) CalibrateGrade(kbType string, rawQuality float64, hasEntityMatch, hasTitleMatch bool) RetrievalGrade {
	p20, p60 := c.getThresholds(kbType)

	if rawQuality >= p20 && hasEntityMatch {
		return GradeA
	}
	if rawQuality >= p20 {
		// High quality but no entity match — could still be B
		return GradeB
	}
	if rawQuality >= p60 {
		return GradeB
	}
	if hasTitleMatch {
		return GradeB
	}
	return GradeC
}

// getThresholds returns P20/P60 for a kb_type, with fallback.
func (c *Calibrator) getThresholds(kbType string) (p20, p60 float64) {
	c.mu.RLock()
	cal, ok := c.byKB[kbType]
	c.mu.RUnlock()
	if ok && cal.SampleCount >= 100 {
		return cal.P20, cal.P60
	}
	return c.FallbackP20, c.FallbackP60
}

// refreshLoop periodically reloads calibration data from the repository.
func (c *Calibrator) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(c.refresh)
	defer ticker.Stop()

	// Initial load
	c.refreshAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refreshAll(ctx)
		}
	}
}

func (c *Calibrator) refreshAll(ctx context.Context) {
	if c.repo == nil {
		return
	}
	// We don't know all kb_types ahead of time; calibration is lazily populated.
	// Periodically refresh the ones we already know about.
	c.mu.RLock()
	known := make([]string, 0, len(c.byKB))
	for k := range c.byKB {
		known = append(known, k)
	}
	c.mu.RUnlock()

	for _, kt := range known {
		c.loadKB(ctx, kt)
	}
}

// LoadKB eagerly loads calibration for a kb_type (called on first use from gate).
func (c *Calibrator) LoadKB(ctx context.Context, kbType string) {
	c.loadKB(ctx, kbType)
}

func (c *Calibrator) loadKB(ctx context.Context, kbType string) {
	if c.repo == nil {
		return
	}
	p20, p60, n, err := c.repo.GetQualityPercentiles(ctx, kbType)
	if err != nil {
		slog.Warn("calibrator: failed to load", "kb_type", kbType, "error", err)
		return
	}
	c.mu.Lock()
	c.byKB[kbType] = &PartitionCalibration{
		KBType:      kbType,
		P20:         p20,
		P60:         p60,
		SampleCount: n,
		UpdatedAt:   time.Now(),
	}
	c.mu.Unlock()
}

// GetPartition returns calibration data for a kb_type (may be nil if not loaded).
func (c *Calibrator) GetPartition(kbType string) *PartitionCalibration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.byKB[kbType]
}
