package engine

import (
	"context"
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func TestStressSmoke(t *testing.T) {
	res := RunStress(StressConfig{
		Events:  500,
		Workers: 2,
		Mix:     StressMixMixed,
		Mode:    StressModeDirect,
	})
	if res.EventsIngested < 500 {
		t.Fatalf("expected at least 500 ingested, got %d", res.EventsIngested)
	}
	if res.TimelineLen == 0 {
		t.Fatal("expected non-empty timeline")
	}
	if res.LiveEvidenceLen == 0 {
		t.Fatal("expected live evidence in ring buffer")
	}
}

func TestStressBusSmoke(t *testing.T) {
	res := RunStress(StressConfig{
		Events:  500,
		Workers: 4,
		Mix:     StressMixLogs,
		Mode:    StressModeBus,
		BusSize: 1024,
	})
	if res.EventsIngested < 500 {
		t.Fatalf("expected at least 500 ingested via bus, got %d", res.EventsIngested)
	}
}

func BenchmarkStress_LogFloodDirect(b *testing.B) {
	st := mockBaseState()
	store := NewStore(&st)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.ApplyEvent(synthLogEvent(i))
	}
}

func BenchmarkStress_LogFlood(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunStress(StressConfig{
			Events:  10_000,
			Workers: 1,
			Mix:     StressMixLogs,
			Mode:    StressModeDirect,
		})
	}
}

func BenchmarkStress_EventCollapse(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunStress(StressConfig{
			Events:  10_000,
			Workers: 1,
			Mix:     StressMixEvents,
			Mode:    StressModeDirect,
		})
	}
}

func BenchmarkStress_MixedBus(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunStress(StressConfig{
			Events:  10_000,
			Workers: 4,
			Mix:     StressMixMixed,
			Mode:    StressModeBus,
			BusSize: 1024,
		})
	}
}

func BenchmarkStress_BusPublish(b *testing.B) {
	bus := NewBus(1024)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var n int
	go bus.RunConsumer(ctx, func(e model.EvidenceEvent) { n++ })
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(synthLogEvent(i))
	}
}

func BenchmarkStress_Duration1s(b *testing.B) {
	if testing.Short() {
		b.Skip("short mode")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunStress(StressConfig{
			Workers:  8,
			Mix:      StressMixMixed,
			Mode:     StressModeBus,
			Duration: 100 * time.Millisecond,
			BusSize:  1024,
		})
	}
}
