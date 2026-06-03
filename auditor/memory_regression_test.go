package auditor

import (
	"testing"

	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
)

func TestMemoryGrowthPct_SteadyGrowth(t *testing.T) {
	step := timeseries.Duration(60)
	start := timeseries.Time(1000)
	points := 30

	ts := timeseries.New(start, points, step)
	for i := 0; i < points; i++ {
		v := float32(100*1024*1024) + float32(i)*float32(2*1024*1024)
		ts.Set(start+timeseries.Time(int64(i)*int64(step)), v)
	}

	to := start + timeseries.Time(int64(points-1)*int64(step))
	pct := MemoryGrowthPct(ts, float32(1024*1024*1024), to)
	assert.Greater(t, pct, float32(0))
	assert.LessOrEqual(t, pct, float32(100))
}

func TestMemoryGrowthPct_EmptyData(t *testing.T) {
	step := timeseries.Duration(60)
	start := timeseries.Time(1000)
	ts := timeseries.New(start, 30, step)

	pct := MemoryGrowthPct(ts, float32(1024*1024*1024), start)
	assert.Equal(t, float32(0), pct)
}

func TestMemoryGrowthPct_ManyNaN(t *testing.T) {
	step := timeseries.Duration(60)
	start := timeseries.Time(1000)
	points := 30

	ts := timeseries.New(start, points, step)
	for i := 0; i < points; i++ {
		if i%3 == 0 {
			v := float32(100*1024*1024) + float32(i)*float32(1*1024*1024)
			ts.Set(start+timeseries.Time(int64(i)*int64(step)), v)
		}
	}

	to := start + timeseries.Time(int64(points-1)*int64(step))
	pct := MemoryGrowthPct(ts, float32(1024*1024*1024), to)
	assert.Equal(t, float32(0), pct)
}

func TestMemoryGrowthPct_DecreasingTrend(t *testing.T) {
	step := timeseries.Duration(60)
	start := timeseries.Time(1000)
	points := 30

	ts := timeseries.New(start, points, step)
	for i := 0; i < points; i++ {
		v := float32(500*1024*1024) - float32(i)*float32(1*1024*1024)
		ts.Set(start+timeseries.Time(int64(i)*int64(step)), v)
	}

	to := start + timeseries.Time(int64(points-1)*int64(step))
	pct := MemoryGrowthPct(ts, float32(1024*1024*1024), to)
	assert.Equal(t, float32(0), pct)
}

func TestMemoryGrowthPct_SuddenDropResetsStreak(t *testing.T) {
	step := timeseries.Duration(60)
	start := timeseries.Time(1000)
	points := 20

	ts := timeseries.New(start, points, step)
	for i := 0; i < points/2; i++ {
		v := float32(100*1024*1024) + float32(i)*float32(3*1024*1024)
		ts.Set(start+timeseries.Time(int64(i)*int64(step)), v)
	}
	for i := points / 2; i < points; i++ {
		v := float32(50*1024*1024) + float32(i-points/2)*float32(2*1024*1024)
		ts.Set(start+timeseries.Time(int64(i)*int64(step)), v)
	}

	to := start + timeseries.Time(int64(points-1)*int64(step))
	pct := MemoryGrowthPct(ts, float32(1024*1024*1024), to)
	assert.Greater(t, pct, float32(0))
}

func TestMemoryGrowthPct_TooFewPoints(t *testing.T) {
	step := timeseries.Duration(60)
	start := timeseries.Time(1000)
	points := 5

	ts := timeseries.New(start, points, step)
	for i := 0; i < points; i++ {
		v := float32(100*1024*1024) + float32(i)*float32(2*1024*1024)
		ts.Set(start+timeseries.Time(int64(i)*int64(step)), v)
	}

	to := start + timeseries.Time(int64(points-1)*int64(step))
	pct := MemoryGrowthPct(ts, float32(1024*1024*1024), to)
	assert.Equal(t, float32(0), pct)
}

func TestMemoryGrowthPct_StableMemory(t *testing.T) {
	step := timeseries.Duration(60)
	start := timeseries.Time(1000)
	points := 30

	ts := timeseries.New(start, points, step)
	for i := 0; i < points; i++ {
		ts.Set(start+timeseries.Time(int64(i)*int64(step)), float32(200*1024*1024))
	}

	to := start + timeseries.Time(int64(points-1)*int64(step))
	pct := MemoryGrowthPct(ts, float32(1024*1024*1024), to)
	assert.Equal(t, float32(0), pct)
}

func TestMemoryGrowthPct_BelowMinGrowth(t *testing.T) {
	step := timeseries.Duration(60)
	start := timeseries.Time(1000)
	points := 30

	ts := timeseries.New(start, points, step)
	for i := 0; i < points; i++ {
		v := float32(100*1024*1024) + float32(i)*float32(100*1024)
		ts.Set(start+timeseries.Time(int64(i)*int64(step)), v)
	}

	to := start + timeseries.Time(int64(points-1)*int64(step))
	pct := MemoryGrowthPct(ts, float32(1024*1024*1024), to)
	assert.Equal(t, float32(0), pct)
}

func TestMemoryGrowthPct_WithLimitExceeded(t *testing.T) {
	step := timeseries.Duration(60)
	start := timeseries.Time(1000)
	points := 30

	ts := timeseries.New(start, points, step)
	for i := 0; i < points; i++ {
		v := float32(500*1024*1024) + float32(i)*float32(10*1024*1024)
		ts.Set(start+timeseries.Time(int64(i)*int64(step)), v)
	}

	to := start + timeseries.Time(int64(points-1)*int64(step))
	pct := MemoryGrowthPct(ts, float32(1024*1024*1024), to)
	assert.Greater(t, pct, float32(0))
}

func TestMemoryGrowthPct_TailGrowthAcceleration(t *testing.T) {
	step := timeseries.Duration(60)
	start := timeseries.Time(1000)
	points := 30

	ts := timeseries.New(start, points, step)
	for i := 0; i < points; i++ {
		v := float32(100*1024*1024) + float32(i)*float32(1*1024*1024)
		if i > points-10 {
			v += float32(i-points+10) * float32(5*1024*1024)
		}
		ts.Set(start+timeseries.Time(int64(i)*int64(step)), v)
	}

	to := start + timeseries.Time(int64(points-1)*int64(step))
	pct := MemoryGrowthPct(ts, float32(1024*1024*1024), to)
	assert.Greater(t, pct, float32(0))
}

func TestMemoryGrowthPct_TailGrowthTooShort(t *testing.T) {
	step := timeseries.Duration(60)
	start := timeseries.Time(1000)
	points := 30

	ts := timeseries.New(start, points, step)
	for i := 0; i < points; i++ {
		v := float32(100*1024*1024) + float32(i)*float32(2*1024*1024)
		if i > points-3 {
			v += float32(i-points+3) * float32(10*1024*1024)
		}
		ts.Set(start+timeseries.Time(int64(i)*int64(step)), v)
	}

	to := start + timeseries.Time(int64(points-1)*int64(step))
	pct := MemoryGrowthPct(ts, float32(1024*1024*1024), to)
	assert.Equal(t, float32(0), pct)
}