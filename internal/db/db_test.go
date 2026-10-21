package db

import (
	"context"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_wuwa.db")

	database, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})
	return database
}

func TestUserCreationAndLookup(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	// 1. Create User
	user, err := database.CreateUser(ctx, "rover", "rover@solaris3.org", "hashed_pwd", "user", 5)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if user.Username != "rover" {
		t.Errorf("Expected username rover, got %s", user.Username)
	}
	if user.QuestionLimit != 5 {
		t.Errorf("Expected question limit 5, got %d", user.QuestionLimit)
	}

	// 2. Duplicate detection
	_, err = database.CreateUser(ctx, "rover", "other@solaris3.org", "pwd", "user", 5)
	if err != ErrUserAlreadyExists {
		t.Errorf("Expected ErrUserAlreadyExists for duplicate username, got %v", err)
	}

	_, err = database.CreateUser(ctx, "rover2", "rover@solaris3.org", "pwd", "user", 5)
	if err != ErrUserAlreadyExists {
		t.Errorf("Expected ErrUserAlreadyExists for duplicate email, got %v", err)
	}

	// 3. Lookup by username or email
	byUsername, err := database.GetUserByUsernameOrEmail(ctx, "rover")
	if err != nil || byUsername.ID != user.ID {
		t.Fatalf("Failed to lookup by username: %v", err)
	}

	byEmail, err := database.GetUserByUsernameOrEmail(ctx, "ROVER@SOLARIS3.ORG")
	if err != nil || byEmail.ID != user.ID {
		t.Fatalf("Failed to lookup by email (case-insensitive): %v", err)
	}

	byID, err := database.GetUserByID(ctx, user.ID)
	if err != nil || byID.Username != "rover" {
		t.Fatalf("Failed to lookup by ID: %v", err)
	}
}

func TestQuestionLimitEnforcement(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	// Create user with limit = 5
	user, err := database.CreateUser(ctx, "yangyang", "yangyang@jinzhoulore.org", "pwd", "user", 5)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// First 5 questions should be allowed
	for i := 1; i <= 5; i++ {
		allowed, remaining, limit, err := database.CheckAndIncrementQuestionCount(ctx, user.ID)
		if err != nil {
			t.Fatalf("CheckAndIncrementQuestionCount iteration %d failed: %v", i, err)
		}
		if !allowed {
			t.Fatalf("Expected iteration %d to be allowed", i)
		}
		expectedRemaining := 5 - i
		if remaining != expectedRemaining {
			t.Errorf("Iteration %d: expected remaining %d, got %d", i, expectedRemaining, remaining)
		}
		if limit != 5 {
			t.Errorf("Iteration %d: expected limit 5, got %d", i, limit)
		}
	}

	// 6th question MUST be rejected
	allowed, remaining, limit, err := database.CheckAndIncrementQuestionCount(ctx, user.ID)
	if err != nil {
		t.Fatalf("6th question check failed with error: %v", err)
	}
	if allowed {
		t.Errorf("Expected 6th question to be rejected, but allowed was true")
	}
	if remaining != 0 {
		t.Errorf("Expected remaining 0 on limit reached, got %d", remaining)
	}
	if limit != 5 {
		t.Errorf("Expected limit 5, got %d", limit)
	}

	// Test Admin reset
	if err := database.ResetUserQuestionCount(ctx, user.ID, 0, 5); err != nil {
		t.Fatalf("ResetUserQuestionCount failed: %v", err)
	}

	// Should be allowed again after reset
	allowed, remaining, _, err = database.CheckAndIncrementQuestionCount(ctx, user.ID)
	if err != nil || !allowed || remaining != 4 {
		t.Fatalf("Expected allowed after reset with remaining=4, got allowed=%v, remaining=%d, err=%v", allowed, remaining, err)
	}
}

func TestAdminExemptFromLimits(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	admin, err := database.CreateUser(ctx, "maintainer", "admin@solaris3.org", "adminpwd", "admin", 5)
	if err != nil {
		t.Fatalf("Failed to create admin: %v", err)
	}

	// Admin asks 12 questions (far exceeding the 5 limit)
	for i := 1; i <= 12; i++ {
		allowed, remaining, _, err := database.CheckAndIncrementQuestionCount(ctx, admin.ID)
		if err != nil {
			t.Fatalf("Admin question check %d failed: %v", i, err)
		}
		if !allowed {
			t.Fatalf("Admin must never be rejected, failed on iteration %d", i)
		}
		if remaining != -1 {
			t.Errorf("Admin remaining quota should be -1 (unlimited), got %d", remaining)
		}
	}
}

func TestThreadAndMessagePersistence(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	user, err := database.CreateUser(ctx, "chixia", "chixia@jinzhoulore.org", "pwd", "user", 5)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// 1. Create Thread
	thread, err := database.CreateThread(ctx, user.ID, "Threnodian Lore Discussion")
	if err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}
	if thread.Title != "Threnodian Lore Discussion" {
		t.Errorf("Unexpected thread title: %s", thread.Title)
	}

	// 2. Add Messages
	msgUser, err := database.AddMessage(ctx, thread.ID, user.ID, "user", "What is an Echo?", "", "")
	if err != nil {
		t.Fatalf("AddMessage (user) failed: %v", err)
	}

	sourcesJSON := `[{"title":"Echoes of Solaris","score":0.89}]`
	metaJSON := `{"model":"wuwa-lore-model","latency_ms":120}`
	msgAssist, err := database.AddMessage(ctx, thread.ID, user.ID, "assistant", "An Echo is a remnant frequency...", sourcesJSON, metaJSON)
	if err != nil {
		t.Fatalf("AddMessage (assistant) failed: %v", err)
	}

	// 3. Get Thread with Messages
	loadedThread, msgs, err := database.GetThreadWithMessages(ctx, user.ID, thread.ID)
	if err != nil {
		t.Fatalf("GetThreadWithMessages failed: %v", err)
	}
	if loadedThread.ID != thread.ID {
		t.Errorf("Loaded thread ID mismatch: %s vs %s", loadedThread.ID, thread.ID)
	}
	if len(msgs) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].ID != msgUser.ID || msgs[1].ID != msgAssist.ID {
		t.Errorf("Messages order or ID mismatch")
	}
	if msgs[1].Sources != sourcesJSON {
		t.Errorf("Sources mismatch: got %s", msgs[1].Sources)
	}

	// 4. List Threads
	threads, err := database.ListThreads(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}
	if len(threads) != 1 || threads[0].ID != thread.ID {
		t.Errorf("ListThreads result mismatch")
	}

	// 5. Delete Thread
	if err := database.DeleteThread(ctx, user.ID, thread.ID); err != nil {
		t.Fatalf("DeleteThread failed: %v", err)
	}

	_, _, err = database.GetThreadWithMessages(ctx, user.ID, thread.ID)
	if err != ErrThreadNotFound {
		t.Errorf("Expected ErrThreadNotFound after deletion, got %v", err)
	}
}
