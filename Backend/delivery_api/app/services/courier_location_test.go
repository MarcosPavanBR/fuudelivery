package services

import (
	"sync"
	"testing"
	"time"
)

// ============================================================================
// Testes de GPS Coordinate Storage/Retrieval
// ============================================================================

func TestCourierStore_UpdateLocation_NewCourier(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	c := store.GetCourier(1)
	if c == nil {
		t.Fatal("GetCourier returned nil for existing courier")
	}
	if c.DeliverymanID != 1 {
		t.Errorf("DeliverymanID: got %d, want 1", c.DeliverymanID)
	}
	if c.Name != "Alice" {
		t.Errorf("Name: got %s, want Alice", c.Name)
	}
	if c.Lat != -23.5505 {
		t.Errorf("Lat: got %f, want -23.5505", c.Lat)
	}
	if c.Lng != -46.6333 {
		t.Errorf("Lng: got %f, want -46.6333", c.Lng)
	}
	if c.Status != "available" {
		t.Errorf("Status: got %s, want available", c.Status)
	}
	if c.CurrentOrders != 0 {
		t.Errorf("CurrentOrders: got %d, want 0", c.CurrentOrders)
	}
	if c.MaxOrders != 3 {
		t.Errorf("MaxOrders: got %d, want 3", c.MaxOrders)
	}
}

func TestCourierStore_UpdateLocation_ExistingCourier(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	// Update location
	store.UpdateLocation(1, "Alice", -23.5600, -46.6400, "busy")

	c := store.GetCourier(1)
	if c == nil {
		t.Fatal("GetCourier returned nil")
	}
	if c.Lat != -23.5600 {
		t.Errorf("Lat not updated: got %f, want -23.5600", c.Lat)
	}
	if c.Lng != -46.6400 {
		t.Errorf("Lng not updated: got %f, want -46.6400", c.Lng)
	}
	if c.Status != "busy" {
		t.Errorf("Status not updated: got %s, want busy", c.Status)
	}
}

func TestCourierStore_UpdateLocation_EmptyNameKeepsOld(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	// Update with empty name should keep old name
	store.UpdateLocation(1, "", -23.5600, -46.6400, "available")

	c := store.GetCourier(1)
	if c == nil {
		t.Fatal("GetCourier returned nil")
	}
	if c.Name != "Alice" {
		t.Errorf("Name should be preserved: got %s, want Alice", c.Name)
	}
}

func TestCourierStore_GetCourier_NonExistent(t *testing.T) {
	store := NewCourierStore()
	c := store.GetCourier(999)
	if c != nil {
		t.Errorf("GetCourier non-existent: got %+v, want nil", c)
	}
}

func TestCourierStore_RemoveCourier(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	store.RemoveCourier(1)

	c := store.GetCourier(1)
	if c != nil {
		t.Error("GetCourier should return nil after RemoveCourier")
	}
}

func TestCourierStore_RemoveCourier_NonExistent(t *testing.T) {
	store := NewCourierStore()
	// Should not panic
	store.RemoveCourier(999)
}

func TestCourierStore_MultipleCouriers(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5500, -46.6330, "available")
	store.UpdateLocation(2, "Bob", -23.5600, -46.6430, "busy")
	store.UpdateLocation(3, "Charlie", -23.5700, -46.6530, "offline")

	if store.GetCourier(1) == nil || store.GetCourier(2) == nil || store.GetCourier(3) == nil {
		t.Error("All couriers should be retrievable")
	}

	c1 := store.GetCourier(1)
	c2 := store.GetCourier(2)
	c3 := store.GetCourier(3)

	if c1.Name != "Alice" || c2.Name != "Bob" || c3.Name != "Charlie" {
		t.Error("Courier names mismatch")
	}
}

// ============================================================================
// Testes de CleanupStale
// ============================================================================

func TestCleanupStale_RemovesOldCouriers(t *testing.T) {
	store := NewCourierStore()

	// Add a courier, then manually set LastUpdate to 2 hours ago
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")
	c := store.GetCourier(1)
	c.LastUpdate = time.Now().UnixMilli() - 7200000 // 2 hours ago

	store.UpdateLocation(2, "Bob", -23.5600, -46.6400, "available")
	// Bob was just updated (recent)

	store.CleanupStale(3600) // 1 hour max age

	if store.GetCourier(1) != nil {
		t.Error("Alice should be removed (stale)")
	}
	if store.GetCourier(2) == nil {
		t.Error("Bob should still exist (recent)")
	}
}

func TestCleanupStale_KeepsRecentCouriers(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")
	store.UpdateLocation(2, "Bob", -23.5600, -46.6400, "available")

	store.CleanupStale(300) // 5 minutes max age

	if store.GetCourier(1) == nil || store.GetCourier(2) == nil {
		t.Error("Both couriers should survive cleanup (recent)")
	}
}

func TestCleanupStale_ZeroAgeRemovesAll(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")
	store.UpdateLocation(2, "Bob", -23.5600, -46.6400, "available")

	store.CleanupStale(0)

	couriers := store.FindNearby(-23.5505, -46.6333, 50.0, 100)
	if len(couriers) != 0 {
		t.Errorf("Expected 0 couriers after 0s cleanup, got %d", len(couriers))
	}
}

func TestCleanupStale_EmptyStore(t *testing.T) {
	store := NewCourierStore()
	// Should not panic
	store.CleanupStale(60)
}

func TestCleanupStale_BoundaryExactAge(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	// Set LastUpdate to exactly 60 seconds ago
	c := store.GetCourier(1)
	c.LastUpdate = time.Now().UnixMilli() - 60000

	store.CleanupStale(60) // exactly 60 seconds max age

	// Courier at exactly the boundary should be removed (LastUpdate <= cutoff)
	if store.GetCourier(1) != nil {
		t.Error("Courier at exact cutoff age should be removed")
	}
}

// ============================================================================
// Testes de Concurrent Location Updates
// ============================================================================

func TestCourierStore_ConcurrentUpdateLocation(t *testing.T) {
	store := NewCourierStore()
	var wg sync.WaitGroup

	// 10 goroutines updating different couriers simultaneously
	for i := int64(0); i < 10; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				store.UpdateLocation(id, "Courier", -23.5505+float64(j)*0.001, -46.6333, "available")
			}
		}(i)
	}

	wg.Wait()

	// All 10 couriers should exist
	for i := int64(0); i < 10; i++ {
		if store.GetCourier(i) == nil {
			t.Errorf("Courier %d should exist after concurrent updates", i)
		}
	}
}

func TestCourierStore_ConcurrentReadWrite(t *testing.T) {
	store := NewCourierStore()

	// Pre-populate
	for i := int64(0); i < 5; i++ {
		store.UpdateLocation(i, "Courier", -23.5505, -46.6333, "available")
	}

	var wg sync.WaitGroup

	// Writers
	for i := int64(0); i < 5; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				store.UpdateLocation(id, "Courier", -23.5505, -46.6333+float64(j)*0.001, "available")
			}
		}(i)
	}

	// Readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				store.FindNearby(-23.5505, -46.6333, 10.0, 5)
			}
		}()
	}

	// Status setters
	for i := int64(0); i < 5; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				store.SetCourierStatus(id, "available")
			}
		}(i)
	}

	// CleanupStale
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 10; j++ {
			store.CleanupStale(60)
		}
	}()

	wg.Wait()
	// No race condition = pass (run with -race)
}

// ============================================================================
// Testes de Online/Offline Status
// ============================================================================

func TestCourierStore_SetCourierStatus_AvailableToOffline(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	store.SetCourierStatus(1, "offline")

	c := store.GetCourier(1)
	if c == nil {
		t.Fatal("Courier should still exist")
	}
	if c.Status != "offline" {
		t.Errorf("Status: got %s, want offline", c.Status)
	}

	// Offline courier should not appear in FindNearby
	couriers := store.FindNearby(-23.5505, -46.6333, 10.0, 10)
	if len(couriers) != 0 {
		t.Error("Offline courier should not appear in FindNearby")
	}
}

func TestCourierStore_SetCourierStatus_OfflineToAvailable(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "offline")

	store.SetCourierStatus(1, "available")

	couriers := store.FindNearby(-23.5505, -46.6333, 10.0, 10)
	if len(couriers) != 1 {
		t.Errorf("Expected 1 courier after going online, got %d", len(couriers))
	}
}

func TestCourierStore_SetCourierStatus_NonExistent(t *testing.T) {
	store := NewCourierStore()
	// Should not panic
	store.SetCourierStatus(999, "available")
}

func TestCourierStore_SetCourierStatus_BusyNotInFindNearby(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	store.SetCourierStatus(1, "busy")

	couriers := store.FindNearby(-23.5505, -46.6333, 10.0, 10)
	for _, c := range couriers {
		if c.DeliverymanID == 1 {
			t.Error("Busy courier should not appear in FindNearby")
		}
	}
}

func TestCourierStore_StatusTransitions(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	statuses := []string{"busy", "offline", "available"}
	for _, status := range statuses {
		store.SetCourierStatus(1, status)
		c := store.GetCourier(1)
		if c == nil {
			t.Fatalf("Courier should exist after setting status %s", status)
		}
		if c.Status != status {
			t.Errorf("Status transition: got %s, want %s", c.Status, status)
		}
	}
}

// ============================================================================
// Testes de FindNearby (complementar ao geo_index_test.go)
// ============================================================================

func TestFindNearby_EmptyStore(t *testing.T) {
	store := NewCourierStore()
	couriers := store.FindNearby(-23.5505, -46.6333, 10.0, 10)
	if len(couriers) != 0 {
		t.Errorf("Empty store: got %d couriers, want 0", len(couriers))
	}
}

func TestFindNearby_Limit(t *testing.T) {
	store := NewCourierStore()

	for i := int64(1); i <= 10; i++ {
		store.UpdateLocation(i, "Courier", -23.5505, -46.6333, "available")
	}

	couriers := store.FindNearby(-23.5505, -46.6333, 10.0, 3)
	if len(couriers) != 3 {
		t.Errorf("FindNearby with limit 3: got %d, want 3", len(couriers))
	}
}

func TestFindNearby_LimitZeroMeansUnlimited(t *testing.T) {
	store := NewCourierStore()

	for i := int64(1); i <= 5; i++ {
		store.UpdateLocation(i, "Courier", -23.5505, -46.6333, "available")
	}

	couriers := store.FindNearby(-23.5505, -46.6333, 10.0, 0)
	if len(couriers) != 5 {
		t.Errorf("FindNearby with limit 0: got %d, want 5", len(couriers))
	}
}

func TestFindNearby_ScoreOrdering(t *testing.T) {
	store := NewCourierStore()

	// Alice is very close, Bob is farther, Charlie is farthest
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")   // same spot
	store.UpdateLocation(2, "Bob", -23.5520, -46.6350, "available")     // ~0.2km
	store.UpdateLocation(3, "Charlie", -23.5550, -46.6380, "available") // ~0.7km

	couriers := store.FindNearby(-23.5505, -46.6333, 10.0, 10)

	if len(couriers) != 3 {
		t.Fatalf("Expected 3 candidates, got %d", len(couriers))
	}
	// First should be Alice (closest = best score)
	if couriers[0].DeliverymanID != 1 {
		t.Errorf("Expected Alice first, got %s (ID=%d)", couriers[0].Name, couriers[0].DeliverymanID)
	}
}

func TestFindNearby_FullCapacityExcluded(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")
	store.SetOrdersCount(1, 3) // MaxOrders=3, at full capacity

	couriers := store.FindNearby(-23.5505, -46.6333, 10.0, 10)
	if len(couriers) != 0 {
		t.Error("Full capacity courier should be excluded")
	}
}

func TestFindNearby_PartialCapacityIncluded(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")
	store.SetOrdersCount(1, 2) // 2/3 orders, still has room

	couriers := store.FindNearby(-23.5505, -46.6333, 10.0, 10)
	if len(couriers) != 1 {
		t.Error("Courier with partial capacity should be included")
	}
}

// ============================================================================
// Testes de CountAvailable
// ============================================================================

func TestCountAvailable_EmptyStore(t *testing.T) {
	store := NewCourierStore()
	count := store.CountAvailable(-23.5505, -46.6333, 10.0)
	if count != 0 {
		t.Errorf("CountAvailable empty store: got %d, want 0", count)
	}
}

func TestCountAvailable_OnlyAvailable(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")
	store.UpdateLocation(2, "Bob", -23.5600, -46.6400, "busy")
	store.UpdateLocation(3, "Charlie", -23.5700, -46.6500, "offline")

	count := store.CountAvailable(-23.5505, -46.6333, 10.0)
	if count != 1 {
		t.Errorf("CountAvailable: got %d, want 1 (only Alice)", count)
	}
}

func TestCountAvailable_ExcludesFarCouriers(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Nearby", -23.5505, -46.6333, "available")
	store.UpdateLocation(2, "FarAway", 51.5074, -0.1278, "available") // London

	count := store.CountAvailable(-23.5505, -46.6333, 10.0)
	if count != 1 {
		t.Errorf("CountAvailable: got %d, want 1 (only Nearby)", count)
	}
}

// ============================================================================
// Testes de Zone Density
// ============================================================================

func TestSetZoneDensity_GetZoneDensity(t *testing.T) {
	store := NewCourierStore()

	store.SetZoneDensity(1, 0.05)
	density := store.GetZoneDensity(1)
	if density != 0.05 {
		t.Errorf("GetZoneDensity: got %f, want 0.05", density)
	}
}

func TestGetZoneDensity_UnsetZone(t *testing.T) {
	store := NewCourierStore()
	density := store.GetZoneDensity(999)
	if density != 0 {
		t.Errorf("GetZoneDensity unset zone: got %f, want 0", density)
	}
}

func TestRecalculateAllDensities(t *testing.T) {
	store := NewCourierStore()

	// Place couriers
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")
	store.UpdateLocation(2, "Bob", -23.5510, -46.6338, "available")

	zones := []ZoneInfo{
		{ID: 1, CenterLat: -23.5505, CenterLng: -46.6333, RadiusKm: 5.0},
		{ID: 2, CenterLat: -23.5505, CenterLng: -46.6333, RadiusKm: 50.0},
	}

	store.RecalculateAllDensities(zones)

	d1 := store.GetZoneDensity(1)
	d2 := store.GetZoneDensity(2)

	if d1 <= 0 {
		t.Errorf("Zone 1 density should be > 0, got %f", d1)
	}
	if d2 <= 0 {
		t.Errorf("Zone 2 density should be > 0, got %f", d2)
	}
	// Zone 1 (smaller area) should have higher density
	if d1 <= d2 {
		t.Errorf("Zone 1 density (%f) should be > zone 2 density (%f)", d1, d2)
	}
}

func TestEstimateDensity_ZeroRadius(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	density := store.EstimateDensity(1, -23.5505, -46.6333, 0)
	if density != 0 {
		t.Errorf("EstimateDensity with zero radius: got %f, want 0", density)
	}
}

// ============================================================================
// Testes de SetOrdersCount
// ============================================================================

func TestSetOrdersCount_ExceedsMax(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")

	// Setting orders beyond MaxOrders should still work (no validation in store)
	store.SetOrdersCount(1, 10)
	c := store.GetCourier(1)
	if c == nil {
		t.Fatal("Courier should exist")
	}
	if c.CurrentOrders != 10 {
		t.Errorf("CurrentOrders: got %d, want 10", c.CurrentOrders)
	}
}

func TestSetOrdersCount_NonExistentCourier(t *testing.T) {
	store := NewCourierStore()
	// Should not panic
	store.SetOrdersCount(999, 5)
}

func TestSetOrdersCount_ResetToZero(t *testing.T) {
	store := NewCourierStore()
	store.UpdateLocation(1, "Alice", -23.5505, -46.6333, "available")
	store.SetOrdersCount(1, 3)
	store.SetOrdersCount(1, 0)

	c := store.GetCourier(1)
	if c == nil {
		t.Fatal("Courier should exist")
	}
	if c.CurrentOrders != 0 {
		t.Errorf("CurrentOrders reset: got %d, want 0", c.CurrentOrders)
	}
}
