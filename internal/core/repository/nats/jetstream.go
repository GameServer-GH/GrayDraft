package nats

import (
	"fmt"
	"time"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
	"github.com/nats-io/nats.go"
)

func ensureStreams(js nats.JetStreamContext, cfg *config.Config) error {
	jsCfg := cfg.NATS.JetStream
	if !jsCfg.Enabled {
		return nil
	}

	maxAge := time.Duration(jsCfg.MaxAgeHours) * time.Hour
	if maxAge <= 0 {
		maxAge = 7 * 24 * time.Hour
	}
	maxBytes := int64(jsCfg.MaxBytes)
	if maxBytes <= 0 {
		maxBytes = 1 << 30
	}
	storage := nats.FileStorage
	if jsCfg.Storage == "memory" {
		storage = nats.MemoryStorage
	}

	subjects := jsCfg.Subjects
	if len(subjects) == 0 {
		subjects = []string{cfg.NATS.UGCSubject}
	}
	if len(subjects) == 0 {
		subjects = []string{"graydraft.ugc.ingest"}
	}

	if err := addOrUpdateStream(js, &nats.StreamConfig{
		Name:      jsCfg.Stream,
		Subjects:  subjects,
		Storage:   storage,
		Retention: nats.LimitsPolicy,
		MaxAge:    maxAge,
		MaxBytes:  maxBytes,
		Discard:   nats.DiscardOld,
		Replicas:  jsCfg.Replicas,
	}); err != nil {
		return fmt.Errorf("stream %s: %w", jsCfg.Stream, err)
	}
	if err := addOrUpdateStream(js, &nats.StreamConfig{Name: "GRAYDRAFT_AGENT_RUNS", Subjects: []string{"graydraft.agent.run", "graydraft.agent.dlq"}, Storage: storage, Retention: nats.LimitsPolicy, MaxAge: maxAge, MaxBytes: maxBytes / 4, Discard: nats.DiscardOld, Replicas: jsCfg.Replicas}); err != nil {
		return fmt.Errorf("agent stream: %w", err)
	}

	// V1-07: when stage_streams_enabled, create a single stream covering all stage subjects.
	// Individual consumers filter per-stage subject.
	if jsCfg.StageStreamsEnabled {
		stageStream := jsCfg.Stream + "_STAGES"
		// Stage subjects must stay disjoint from GRAYDRAFT_UGC (graydraft.ugc.ingest).
		// Legacy ingest stays on the main stream; staged ingest publishes graydraft.ingest.*.
		stageSubjects := model.AllStageSubjects()
		if err := addOrUpdateStream(js, &nats.StreamConfig{
			Name:      stageStream,
			Subjects:  stageSubjects,
			Storage:   storage,
			Retention: nats.LimitsPolicy,
			MaxAge:    maxAge,
			MaxBytes:  maxBytes,
			Discard:   nats.DiscardOld,
			Replicas:  jsCfg.Replicas,
		}); err != nil {
			return fmt.Errorf("stage stream %s: %w", stageStream, err)
		}
	}

	if jsCfg.DLQStream == "" {
		return nil
	}
	dlqSubjects := jsCfg.DLQSubjects
	if len(dlqSubjects) == 0 && jsCfg.DLQSubject != "" {
		dlqSubjects = []string{jsCfg.DLQSubject}
	}
	if len(dlqSubjects) == 0 {
		dlqSubjects = []string{"graydraft.ugc.dlq"}
	}
	if err := addOrUpdateStream(js, &nats.StreamConfig{
		Name:      jsCfg.DLQStream,
		Subjects:  dlqSubjects,
		Storage:   storage,
		Retention: nats.LimitsPolicy,
		MaxAge:    maxAge,
		MaxBytes:  maxBytes / 4,
		Discard:   nats.DiscardOld,
		Replicas:  jsCfg.Replicas,
	}); err != nil {
		return fmt.Errorf("dlq stream %s: %w", jsCfg.DLQStream, err)
	}

	// V1-11: per-stage DLQ subjects when stage streams are enabled.
	if jsCfg.StageStreamsEnabled && jsCfg.DLQStream != "" {
		stageDLQStream := jsCfg.DLQStream + "_STAGES"
		stageDLQSubjects := make([]string, len(model.StageOrder))
		for i, s := range model.StageOrder {
			stageDLQSubjects[i] = "graydraft.ingest.dlq." + string(s)
		}
		if err := addOrUpdateStream(js, &nats.StreamConfig{
			Name:      stageDLQStream,
			Subjects:  stageDLQSubjects,
			Storage:   storage,
			Retention: nats.LimitsPolicy,
			MaxAge:    maxAge,
			MaxBytes:  maxBytes / 2,
			Discard:   nats.DiscardOld,
			Replicas:  jsCfg.Replicas,
		}); err != nil {
			return fmt.Errorf("stage dlq stream %s: %w", stageDLQStream, err)
		}
	}

	// V1-16: cache invalidation stream.
	if jsCfg.StageStreamsEnabled || cfg.Cache.FineGrainedInvalidation {
		invalStream := "GRAYDRAFT_CACHE_INVAL"
		invalSubject := cfg.Cache.InvalidationSubject
		if invalSubject == "" {
			invalSubject = "graydraft.cache.invalidate"
		}
		if err := addOrUpdateStream(js, &nats.StreamConfig{
			Name:      invalStream,
			Subjects:  []string{invalSubject},
			Storage:   storage,
			Retention: nats.LimitsPolicy,
			MaxAge:    1 * time.Hour,
			MaxBytes:  maxBytes / 4,
			Discard:   nats.DiscardOld,
			Replicas:  jsCfg.Replicas,
		}); err != nil {
			return fmt.Errorf("invalidation stream %s: %w", invalStream, err)
		}
	}

	return nil
}

func addOrUpdateStream(js nats.JetStreamContext, desired *nats.StreamConfig) error {
	info, err := js.StreamInfo(desired.Name)
	if err != nil {
		if err != nats.ErrStreamNotFound {
			return err
		}
		_, err = js.AddStream(desired)
		return err
	}
	cur := info.Config
	cur.Subjects = desired.Subjects
	cur.Storage = desired.Storage
	cur.Retention = desired.Retention
	cur.MaxAge = desired.MaxAge
	cur.MaxBytes = desired.MaxBytes
	cur.Discard = desired.Discard
	cur.Replicas = desired.Replicas
	_, err = js.UpdateStream(&cur)
	return err
}
