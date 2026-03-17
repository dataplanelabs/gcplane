package controller

import (
	"sync"
	"testing"
	"time"
)

func TestNewStatusTracker_StartsEmpty(t *testing.T) {
	tracker := NewStatusTracker()
	s := tracker.Get()
	if len(s.Conditions) != 0 {
		t.Errorf("expected 0 conditions, got %d", len(s.Conditions))
	}
	if len(s.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(s.Resources))
	}
}

func TestStatusTracker_UpdateAndGet(t *testing.T) {
	tracker := NewStatusTracker()
	now := time.Now()
	tracker.Update(SyncStatus{
		LastSyncHash: "abc123",
		LastSyncTime: now,
	})

	s := tracker.Get()
	if s.LastSyncHash != "abc123" {
		t.Errorf("expected hash abc123, got %s", s.LastSyncHash)
	}
}

func TestStatusTracker_Get_ReturnsIndependentCopy(t *testing.T) {
	tracker := NewStatusTracker()
	tracker.Update(SyncStatus{
		Conditions: []Condition{
			{Type: ConditionSynced, Status: "True"},
		},
	})

	copy1 := tracker.Get()
	copy1.Conditions[0].Status = "False"

	copy2 := tracker.Get()
	if copy2.Conditions[0].Status != "True" {
		t.Error("modifying returned copy affected tracker state")
	}
}

func TestStatusTracker_SetCondition_Upsert(t *testing.T) {
	tracker := NewStatusTracker()

	tracker.SetCondition(Condition{Type: ConditionSynced, Status: "False", Message: "initial"})
	tracker.SetCondition(Condition{Type: ConditionError, Status: "False"})
	tracker.SetCondition(Condition{Type: ConditionSynced, Status: "True", Message: "updated"})

	s := tracker.Get()
	if len(s.Conditions) != 2 {
		t.Errorf("expected 2 conditions after upsert, got %d", len(s.Conditions))
	}

	var synced *Condition
	for i := range s.Conditions {
		if s.Conditions[i].Type == ConditionSynced {
			synced = &s.Conditions[i]
		}
	}
	if synced == nil {
		t.Fatal("Synced condition not found")
	}
	if synced.Status != "True" {
		t.Errorf("expected Synced=True, got %s", synced.Status)
	}
	if synced.Message != "updated" {
		t.Errorf("expected message 'updated', got %s", synced.Message)
	}
}

func TestStatusTracker_SetCondition_LastTransitionTime_OnStatusChange(t *testing.T) {
	tracker := NewStatusTracker()

	tracker.SetCondition(Condition{Type: ConditionSynced, Status: "False"})
	s1 := tracker.Get()
	t1 := s1.Conditions[0].LastTransitionTime

	// Short sleep to ensure time advances
	time.Sleep(2 * time.Millisecond)

	// Status changes: LastTransitionTime must update
	tracker.SetCondition(Condition{Type: ConditionSynced, Status: "True"})
	s2 := tracker.Get()
	t2 := s2.Conditions[0].LastTransitionTime

	if !t2.After(t1) {
		t.Error("expected LastTransitionTime to advance when Status changes")
	}
}

func TestStatusTracker_SetCondition_LastTransitionTime_NoChangeOnSameStatus(t *testing.T) {
	tracker := NewStatusTracker()

	tracker.SetCondition(Condition{Type: ConditionSynced, Status: "True"})
	s1 := tracker.Get()
	t1 := s1.Conditions[0].LastTransitionTime

	time.Sleep(2 * time.Millisecond)

	// Same status: LastTransitionTime must NOT update
	tracker.SetCondition(Condition{Type: ConditionSynced, Status: "True", Message: "new message"})
	s2 := tracker.Get()
	t2 := s2.Conditions[0].LastTransitionTime

	if !t2.Equal(t1) {
		t.Error("expected LastTransitionTime to stay the same when Status is unchanged")
	}
}

func TestStatusTracker_IsSynced_True(t *testing.T) {
	tracker := NewStatusTracker()
	tracker.SetCondition(Condition{Type: ConditionSynced, Status: "True"})
	if !tracker.IsSynced() {
		t.Error("expected IsSynced=true")
	}
}

func TestStatusTracker_IsSynced_False(t *testing.T) {
	tracker := NewStatusTracker()
	tracker.SetCondition(Condition{Type: ConditionSynced, Status: "False"})
	if tracker.IsSynced() {
		t.Error("expected IsSynced=false")
	}
}

func TestStatusTracker_IsSynced_NoConditions(t *testing.T) {
	tracker := NewStatusTracker()
	if tracker.IsSynced() {
		t.Error("expected IsSynced=false with no conditions")
	}
}

func TestStatusTracker_ConcurrentReadWrite(t *testing.T) {
	tracker := NewStatusTracker()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			tracker.SetCondition(Condition{Type: ConditionSynced, Status: "True"})
		}()
		go func() {
			defer wg.Done()
			_ = tracker.IsSynced()
			_ = tracker.Get()
		}()
	}

	wg.Wait()
}
