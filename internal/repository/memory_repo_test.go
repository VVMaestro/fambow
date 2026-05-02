package repository

import (
	"context"
	"testing"
)

func TestMemoryRepositoryNextRandomMemoryForUserReturnsUniqueUntilExhausted(t *testing.T) {
	ctx := context.Background()
	db := openLoveNoteTestDB(t)
	repo := NewMemoryRepository(db)
	telegramUserID := seedUser(t, db, 101, "Anna", UserTypeWife)

	for _, text := range []string{"First", "Second", "Third"} {
		if _, err := repo.SaveMemory(ctx, telegramUserID, "Anna", text, "", "", nil); err != nil {
			t.Fatalf("SaveMemory(%q) unexpected error: %v", text, err)
		}
	}

	seen := make(map[int64]struct{})
	for range 3 {
		memory, err := repo.NextRandomMemoryForUser(ctx, telegramUserID)
		if err != nil {
			t.Fatalf("NextRandomMemoryForUser() unexpected error: %v", err)
		}
		if _, exists := seen[memory.ID]; exists {
			t.Fatalf("received duplicate memory before exhaustion: %#v", memory)
		}
		seen[memory.ID] = struct{}{}
	}

	if len(seen) != 3 {
		t.Fatalf("expected 3 unique memories, got %d", len(seen))
	}

	var cycleCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_user_cycle WHERE user_id = (SELECT id FROM users WHERE telegram_user_id = ?)`, telegramUserID).Scan(&cycleCount); err != nil {
		t.Fatalf("count memory cycle rows: %v", err)
	}
	if cycleCount != 3 {
		t.Fatalf("expected 3 cycle rows after exhaustion, got %d", cycleCount)
	}
}

func TestMemoryRepositoryNextRandomMemoryForUserResetsAfterExhaustion(t *testing.T) {
	ctx := context.Background()
	db := openLoveNoteTestDB(t)
	repo := NewMemoryRepository(db)
	telegramUserID := seedUser(t, db, 101, "Anna", UserTypeWife)

	for _, text := range []string{"First", "Second"} {
		if _, err := repo.SaveMemory(ctx, telegramUserID, "Anna", text, "", "", nil); err != nil {
			t.Fatalf("SaveMemory(%q) unexpected error: %v", text, err)
		}
	}

	firstCycle := make(map[int64]struct{})
	for range 2 {
		memory, err := repo.NextRandomMemoryForUser(ctx, telegramUserID)
		if err != nil {
			t.Fatalf("NextRandomMemoryForUser() unexpected error: %v", err)
		}
		firstCycle[memory.ID] = struct{}{}
	}

	memory, err := repo.NextRandomMemoryForUser(ctx, telegramUserID)
	if err != nil {
		t.Fatalf("NextRandomMemoryForUser(reset) unexpected error: %v", err)
	}
	if _, ok := firstCycle[memory.ID]; !ok {
		t.Fatalf("expected reset memory from existing pool, got %#v", memory)
	}

	var cycleCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_user_cycle WHERE user_id = (SELECT id FROM users WHERE telegram_user_id = ?)`, telegramUserID).Scan(&cycleCount); err != nil {
		t.Fatalf("count memory cycle rows after reset: %v", err)
	}
	if cycleCount != 1 {
		t.Fatalf("expected cycle to restart with 1 row, got %d", cycleCount)
	}
}

func TestMemoryRepositoryNextRandomMemoryForUserIncludesNewItemsMidCycle(t *testing.T) {
	ctx := context.Background()
	db := openLoveNoteTestDB(t)
	repo := NewMemoryRepository(db)
	telegramUserID := seedUser(t, db, 101, "Anna", UserTypeWife)

	for _, text := range []string{"First", "Second"} {
		if _, err := repo.SaveMemory(ctx, telegramUserID, "Anna", text, "", "", nil); err != nil {
			t.Fatalf("SaveMemory(%q) unexpected error: %v", text, err)
		}
	}

	first, err := repo.NextRandomMemoryForUser(ctx, telegramUserID)
	if err != nil {
		t.Fatalf("NextRandomMemoryForUser(first) unexpected error: %v", err)
	}

	if _, err := repo.SaveMemory(ctx, telegramUserID, "Anna", "Third", "", "", nil); err != nil {
		t.Fatalf("SaveMemory(third) unexpected error: %v", err)
	}

	second, err := repo.NextRandomMemoryForUser(ctx, telegramUserID)
	if err != nil {
		t.Fatalf("NextRandomMemoryForUser(second) unexpected error: %v", err)
	}
	third, err := repo.NextRandomMemoryForUser(ctx, telegramUserID)
	if err != nil {
		t.Fatalf("NextRandomMemoryForUser(third) unexpected error: %v", err)
	}

	seen := map[int64]struct{}{
		first.ID:  {},
		second.ID: {},
		third.ID:  {},
	}
	if len(seen) != 3 {
		t.Fatalf("expected newly added memory to appear before reset, got ids %#v", seen)
	}
}

func TestMemoryRepositoryNextRandomMemoryForUserKeepsCyclesIndependent(t *testing.T) {
	ctx := context.Background()
	db := openLoveNoteTestDB(t)
	repo := NewMemoryRepository(db)
	firstUser := seedUser(t, db, 101, "Anna", UserTypeWife)
	secondUser := seedUser(t, db, 202, "Mia", UserTypeWife)

	for _, text := range []string{"First", "Second"} {
		if _, err := repo.SaveMemory(ctx, firstUser, "Anna", text, "", "", nil); err != nil {
			t.Fatalf("SaveMemory(%q) unexpected error: %v", text, err)
		}
	}

	firstMemory, err := repo.NextRandomMemoryForUser(ctx, firstUser)
	if err != nil {
		t.Fatalf("NextRandomMemoryForUser(first user) unexpected error: %v", err)
	}
	secondMemory, err := repo.NextRandomMemoryForUser(ctx, secondUser)
	if err != nil {
		t.Fatalf("NextRandomMemoryForUser(second user) unexpected error: %v", err)
	}

	var firstCycleCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_user_cycle WHERE user_id = (SELECT id FROM users WHERE telegram_user_id = ?)`, firstUser).Scan(&firstCycleCount); err != nil {
		t.Fatalf("count first user memory cycle rows: %v", err)
	}
	var secondCycleCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_user_cycle WHERE user_id = (SELECT id FROM users WHERE telegram_user_id = ?)`, secondUser).Scan(&secondCycleCount); err != nil {
		t.Fatalf("count second user memory cycle rows: %v", err)
	}
	if firstCycleCount != 1 || secondCycleCount != 1 {
		t.Fatalf("expected independent memory cycles with 1 row each, got first=%d second=%d", firstCycleCount, secondCycleCount)
	}
	if firstMemory.ID == 0 || secondMemory.ID == 0 {
		t.Fatalf("expected both users to receive memories, got first=%#v second=%#v", firstMemory, secondMemory)
	}
}
