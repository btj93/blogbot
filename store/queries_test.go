package store

import (
	"context"
	"strings"
	"testing"
)

func TestGroupCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	g, err := s.GetOrCreateGroup(ctx, "乃木坂46")
	if err != nil {
		t.Fatal(err)
	}

	if g.Name != "乃木坂46" {
		t.Errorf("got name=%q", g.Name)
	}
	// Idempotent
	g2, err := s.GetOrCreateGroup(ctx, "乃木坂46")
	if err != nil {
		t.Fatal(err)
	}

	if g2.ID != g.ID {
		t.Error("got different ID on second insert")
	}

	groups, err := s.ListGroups(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(groups) != 1 {
		t.Errorf("got %d groups", len(groups))
	}
}

func TestMemberCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	g, _ := s.GetOrCreateGroup(ctx, "乃木坂46")
	gen4 := 4

	m, err := s.GetOrCreateMember(ctx, g.ID, "遠藤さくら", &gen4, false)
	if err != nil {
		t.Fatal(err)
	}

	if m.Name != "遠藤さくら" || *m.Generation != 4 || m.Disabled {
		t.Errorf("unexpected member: %+v", m)
	}

	_, err = s.GetOrCreateMember(ctx, g.ID, "運営スタッフ", nil, true)
	if err != nil {
		t.Fatal(err)
	}

	members, err := s.ListMembersByGroup(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(members) != 2 {
		t.Errorf("got %d members", len(members))
	}

	enabled, err := s.ListEnabledMembersByGroupAndGeneration(ctx, g.ID, &gen4)
	if err != nil {
		t.Fatal(err)
	}

	if len(enabled) != 1 || enabled[0].Name != "遠藤さくら" {
		t.Errorf("got enabled=%+v", enabled)
	}

	nullEnabled, err := s.ListEnabledMembersByGroupAndGeneration(ctx, g.ID, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(nullEnabled) != 0 {
		t.Errorf("got null-gen enabled=%d, want 0 (staff is disabled)", len(nullEnabled))
	}

	gens, err := s.ListGenerationsForGroup(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(gens) != 1 || *gens[0] != 4 {
		t.Errorf("got gens=%+v", gens)
	}
}

func TestSubscriptionCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	g, _ := s.GetOrCreateGroup(ctx, "乃木坂46")
	gen4 := 4
	m, _ := s.GetOrCreateMember(ctx, g.ID, "遠藤さくら", &gen4, false)

	added, err := s.AddSubscription(ctx, m.ID, "12345")
	if err != nil {
		t.Fatal(err)
	}

	if !added {
		t.Error("expected added=true")
	}

	added2, err := s.AddSubscription(ctx, m.ID, "12345")
	if err != nil {
		t.Fatal(err)
	}

	if added2 {
		t.Error("expected added=false on duplicate")
	}

	ok, _ := s.IsSubscribed(ctx, m.ID, "12345")
	if !ok {
		t.Error("expected subscribed")
	}

	chatIDs, _ := s.GetSubscriberChatIDs(ctx, m.ID)
	if len(chatIDs) != 1 || chatIDs[0] != "12345" {
		t.Errorf("got chatIDs=%v", chatIDs)
	}

	removed, _ := s.RemoveSubscription(ctx, m.ID, "12345")
	if !removed {
		t.Error("expected removed=true")
	}

	ok2, _ := s.IsSubscribed(ctx, m.ID, "12345")
	if ok2 {
		t.Error("expected not subscribed after remove")
	}
}

func TestSubscriptionBulkByGeneration(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	g, _ := s.GetOrCreateGroup(ctx, "乃木坂46")
	gen4 := 4
	_, _ = s.GetOrCreateMember(ctx, g.ID, "遠藤さくら", &gen4, false)
	_, _ = s.GetOrCreateMember(ctx, g.ID, "賀喜遥香", &gen4, false)
	_, _ = s.GetOrCreateMember(ctx, g.ID, "運営スタッフ", &gen4, true) // disabled

	err := s.AddSubscriptionForAllInGeneration(ctx, g.ID, &gen4, "12345")
	if err != nil {
		t.Fatal(err)
	}

	m1, _ := s.GetMemberByGroupAndName(ctx, g.ID, "遠藤さくら")
	m2, _ := s.GetMemberByGroupAndName(ctx, g.ID, "賀喜遥香")
	m3, _ := s.GetMemberByGroupAndName(ctx, g.ID, "運営スタッフ")
	ok1, _ := s.IsSubscribed(ctx, m1.ID, "12345")
	ok2, _ := s.IsSubscribed(ctx, m2.ID, "12345")
	ok3, _ := s.IsSubscribed(ctx, m3.ID, "12345")

	if !ok1 || !ok2 {
		t.Error("expected enabled members to be subscribed")
	}

	if ok3 {
		t.Error("disabled member should not be subscribed")
	}

	err = s.RemoveSubscriptionForAllInGeneration(ctx, g.ID, &gen4, "12345")
	if err != nil {
		t.Fatal(err)
	}

	ok1, _ = s.IsSubscribed(ctx, m1.ID, "12345")
	if ok1 {
		t.Error("expected unsubscribed after remove all")
	}
}

func TestBlogProgress(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	claimed, err := s.ClaimBlog(ctx, "https://example.com/blog/1")
	if err != nil {
		t.Fatal(err)
	}

	if !claimed {
		t.Error("expected first claim to succeed")
	}

	claimed2, err := s.ClaimBlog(ctx, "https://example.com/blog/1")
	if err != nil {
		t.Fatal(err)
	}

	if claimed2 {
		t.Error("expected second claim to fail (already claimed)")
	}
}

func TestShowroomRoomCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	g, _ := s.GetOrCreateGroup(ctx, "乃木坂46")
	gen4 := 4
	m, _ := s.GetOrCreateMember(ctx, g.ID, "遠藤さくら", &gen4, false)

	err := s.UpsertShowroomRoom(ctx, m.ID, "room123", "https://www.showroom-live.com/example")
	if err != nil {
		t.Fatal(err)
	}

	rooms, err := s.ListShowroomRoomsWithRoomID(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(rooms) != 1 || rooms[0].RoomID != "room123" {
		t.Errorf("got rooms=%+v", rooms)
	}

	epoch := int64(1700000000)
	text := "2024-01-01 20:00"

	err = s.UpdateNextLive(ctx, m.ID, &epoch, &text)
	if err != nil {
		t.Fatal(err)
	}

	rooms2, _ := s.ListShowroomRoomsWithRoomID(ctx)
	if *rooms2[0].NextLiveEpoch != epoch || *rooms2[0].NextLiveText != text {
		t.Errorf("got next_live=%v/%v", rooms2[0].NextLiveEpoch, rooms2[0].NextLiveText)
	}
}

func TestForeignKeyEnforcement(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Attempt to subscribe to a nonexistent member (ID 9999).
	// The subscriptions table has a foreign key on member_id referencing members(id),
	// so this should fail.
	_, err := s.AddSubscription(ctx, 9999, "12345")
	if err == nil {
		t.Fatal("expected foreign key error when subscribing to nonexistent member, got nil")
	}

	if !strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Errorf("expected FOREIGN KEY error, got: %v", err)
	}
}

func TestBlogProgressUniqueURL(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	url := "https://example.com/blog/unique-test"

	// First claim should succeed.
	claimed, err := s.ClaimBlog(ctx, url)
	if err != nil {
		t.Fatal(err)
	}

	if !claimed {
		t.Fatal("expected first claim to succeed")
	}

	// Second claim of the same URL should return false.
	claimed2, err := s.ClaimBlog(ctx, url)
	if err != nil {
		t.Fatal(err)
	}

	if claimed2 {
		t.Fatal("expected second claim to return false")
	}

	// Verify there is exactly one row for this URL, not two.
	var count int

	err = s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM blog_progress WHERE url = $1", url).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Errorf("expected exactly 1 row for URL, got %d", count)
	}
}

func TestNullableGeneration(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	g, _ := s.GetOrCreateGroup(ctx, "乃木坂46")

	// Create a member with nil generation, enabled (not disabled).
	m, err := s.GetOrCreateMember(ctx, g.ID, "テストメンバー", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	if m.Generation != nil {
		t.Errorf("expected nil generation, got %v", m.Generation)
	}

	// ListGenerationsForGroup should include nil.
	gens, err := s.ListGenerationsForGroup(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}

	foundNil := false

	for _, gen := range gens {
		if gen == nil {
			foundNil = true
			break
		}
	}

	if !foundNil {
		t.Errorf("expected nil generation in list, got %+v", gens)
	}

	// ListEnabledMembersByGroupAndGeneration with nil should return the member.
	members, err := s.ListEnabledMembersByGroupAndGeneration(ctx, g.ID, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(members) != 1 {
		t.Fatalf("expected 1 member with nil generation, got %d", len(members))
	}

	if members[0].Name != "テストメンバー" {
		t.Errorf("expected テストメンバー, got %q", members[0].Name)
	}

	if members[0].Generation != nil {
		t.Errorf("expected nil generation on returned member, got %v", members[0].Generation)
	}
}

func TestUpdateNextLiveNullRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	g, _ := s.GetOrCreateGroup(ctx, "乃木坂46")
	gen4 := 4
	m, _ := s.GetOrCreateMember(ctx, g.ID, "遠藤さくら", &gen4, false)

	// Create a showroom room.
	err := s.UpsertShowroomRoom(ctx, m.ID, "room456", "https://www.showroom-live.com/test")
	if err != nil {
		t.Fatal(err)
	}

	// Set epoch and text to non-nil values.
	epoch := int64(1700000000)
	text := "2024-01-01 20:00"

	err = s.UpdateNextLive(ctx, m.ID, &epoch, &text)
	if err != nil {
		t.Fatal(err)
	}

	// Verify they are set.
	rooms, err := s.ListShowroomRoomsWithRoomID(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(rooms) != 1 {
		t.Fatalf("expected 1 room, got %d", len(rooms))
	}

	if rooms[0].NextLiveEpoch == nil || *rooms[0].NextLiveEpoch != epoch {
		t.Errorf("expected epoch=%d, got %v", epoch, rooms[0].NextLiveEpoch)
	}

	if rooms[0].NextLiveText == nil || *rooms[0].NextLiveText != text {
		t.Errorf("expected text=%q, got %v", text, rooms[0].NextLiveText)
	}

	// Clear to nil.
	err = s.UpdateNextLive(ctx, m.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Verify they are nil.
	rooms2, err := s.ListShowroomRoomsWithRoomID(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(rooms2) != 1 {
		t.Fatalf("expected 1 room, got %d", len(rooms2))
	}

	if rooms2[0].NextLiveEpoch != nil {
		t.Errorf("expected nil epoch after clear, got %v", rooms2[0].NextLiveEpoch)
	}

	if rooms2[0].NextLiveText != nil {
		t.Errorf("expected nil text after clear, got %v", rooms2[0].NextLiveText)
	}
}

func TestBulkSubscriptionByGeneration(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	g, _ := s.GetOrCreateGroup(ctx, "乃木坂46")

	gen1 := 1
	gen2 := 2

	// Create members in gen1.
	m1, _ := s.GetOrCreateMember(ctx, g.ID, "メンバーA", &gen1, false)
	m2, _ := s.GetOrCreateMember(ctx, g.ID, "メンバーB", &gen1, false)

	// Create members in gen2.
	m3, _ := s.GetOrCreateMember(ctx, g.ID, "メンバーC", &gen2, false)
	m4, _ := s.GetOrCreateMember(ctx, g.ID, "メンバーD", &gen2, false)

	chatID := "chat999"

	// Subscribe all in gen1.
	err := s.AddSubscriptionForAllInGeneration(ctx, g.ID, &gen1, chatID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify gen1 members are subscribed.
	ok1, _ := s.IsSubscribed(ctx, m1.ID, chatID)

	ok2, _ := s.IsSubscribed(ctx, m2.ID, chatID)
	if !ok1 || !ok2 {
		t.Error("expected gen1 members to be subscribed")
	}

	// Verify gen2 members are NOT subscribed.
	ok3, _ := s.IsSubscribed(ctx, m3.ID, chatID)

	ok4, _ := s.IsSubscribed(ctx, m4.ID, chatID)
	if ok3 || ok4 {
		t.Error("expected gen2 members to NOT be subscribed")
	}

	// Remove subscriptions for gen1.
	err = s.RemoveSubscriptionForAllInGeneration(ctx, g.ID, &gen1, chatID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify gen1 members are no longer subscribed.
	ok1, _ = s.IsSubscribed(ctx, m1.ID, chatID)

	ok2, _ = s.IsSubscribed(ctx, m2.ID, chatID)
	if ok1 || ok2 {
		t.Error("expected gen1 members to be unsubscribed after removal")
	}
}

func TestEmptyResults(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	g, _ := s.GetOrCreateGroup(ctx, "乃木坂46")

	// ListMembersByGroup for a group with no members should return empty slice.
	members, err := s.ListMembersByGroup(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(members) != 0 {
		t.Errorf("expected 0 members for empty group, got %d", len(members))
	}

	// GetSubscriberChatIDs for a nonexistent member should return empty slice.
	chatIDs, err := s.GetSubscriberChatIDs(ctx, 9999)
	if err != nil {
		t.Fatal(err)
	}

	if len(chatIDs) != 0 {
		t.Errorf("expected 0 chatIDs for nonexistent member, got %d", len(chatIDs))
	}
}

func TestClaimBlogDedup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	url := "https://example.com/blog/claim-dedup"

	// First claim should succeed.
	claimed, err := s.ClaimBlog(ctx, url)
	if err != nil {
		t.Fatal(err)
	}

	if !claimed {
		t.Fatal("expected first claim to succeed")
	}

	// Second claim should return false (already claimed).
	claimed2, err := s.ClaimBlog(ctx, url)
	if err != nil {
		t.Fatal(err)
	}

	if claimed2 {
		t.Fatal("expected second claim to return false")
	}
}

func TestAcquireLock(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// First acquire should succeed.
	lockID, err := s.AcquireLock(ctx, "-100", 42, "Alice Smith")
	if err != nil {
		t.Fatal(err)
	}

	if lockID == "" {
		t.Fatal("expected non-empty lock_id")
	}

	// Same user re-acquire should return a lock_id (idempotent).
	lockID2, err := s.AcquireLock(ctx, "-100", 42, "Alice Smith")
	if err != nil {
		t.Fatal(err)
	}

	if lockID2 == "" {
		t.Fatal("expected non-empty lock_id on re-acquire by same user")
	}

	// Different user should get empty string (locked).
	lockID3, err := s.AcquireLock(ctx, "-100", 99, "Bob Lee")
	if err != nil {
		t.Fatal(err)
	}

	if lockID3 != "" {
		t.Errorf("expected empty lock_id when locked by another user, got %q", lockID3)
	}
}

func TestAcquireLock_DifferentChats(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	lockID1, err := s.AcquireLock(ctx, "-100", 42, "Alice Smith")
	if err != nil {
		t.Fatal(err)
	}

	if lockID1 == "" {
		t.Fatal("expected lock for chat -100")
	}

	// Different chat should succeed independently.
	lockID2, err := s.AcquireLock(ctx, "-200", 99, "Bob Lee")
	if err != nil {
		t.Fatal(err)
	}

	if lockID2 == "" {
		t.Fatal("expected lock for chat -200")
	}
}

func TestGetLockHolder(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// No lock — empty string.
	holder, err := s.GetLockHolder(ctx, "-100")
	if err != nil {
		t.Fatal(err)
	}

	if holder != "" {
		t.Errorf("expected empty holder, got %q", holder)
	}

	// Acquire and check holder.
	_, err = s.AcquireLock(ctx, "-100", 42, "Alice Smith")
	if err != nil {
		t.Fatal(err)
	}

	holder, err = s.GetLockHolder(ctx, "-100")
	if err != nil {
		t.Fatal(err)
	}

	if holder != "Alice Smith" {
		t.Errorf("expected holder 'Alice Smith', got %q", holder)
	}
}

func TestAcquireLock_Expired(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Insert an already-expired lock directly.
	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO webapp_locks (chat_id, user_id, user_name, lock_id, expires_at)
		 VALUES ($1, $2, $3, gen_random_uuid(), NOW() - INTERVAL '1 minute')`,
		"-100", 42, "Alice Smith",
	)
	if err != nil {
		t.Fatal(err)
	}

	// A different user should be able to acquire it (expired lock is cleaned up).
	lockID, err := s.AcquireLock(ctx, "-100", 99, "Bob Lee")
	if err != nil {
		t.Fatal(err)
	}

	if lockID == "" {
		t.Fatal("expected to acquire lock after previous expired")
	}

	// Verify the holder is now Bob.
	holder, err := s.GetLockHolder(ctx, "-100")
	if err != nil {
		t.Fatal(err)
	}

	if holder != "Bob Lee" {
		t.Errorf("expected holder 'Bob Lee', got %q", holder)
	}
}

func TestValidateAndReleaseLock(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	lockID, err := s.AcquireLock(ctx, "-100", 42, "Alice Smith")
	if err != nil {
		t.Fatal(err)
	}

	// Validate and release within a transaction.
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = s.ValidateAndReleaseLock(ctx, tx, "-100", lockID)
	if err != nil {
		_ = tx.Rollback()

		t.Fatal(err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	// Lock should be gone.
	holder, err := s.GetLockHolder(ctx, "-100")
	if err != nil {
		t.Fatal(err)
	}

	if holder != "" {
		t.Errorf("expected no holder after release, got %q", holder)
	}
}

func TestValidateAndReleaseLock_InvalidLock(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// No lock exists at all — should reject (invalid lock_id).
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = s.ValidateAndReleaseLock(ctx, tx, "-100", "00000000-0000-0000-0000-000000000000")
	_ = tx.Rollback()

	if err == nil {
		t.Fatal("expected error for invalid lock_id when no lock exists")
	}

	if err.Error() != "invalid lock" {
		t.Errorf("expected 'invalid lock' error, got: %v", err)
	}
}

func TestValidateAndReleaseLock_StolenByOther(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Alice acquires.
	_, err := s.AcquireLock(ctx, "-100", 42, "Alice Smith")
	if err != nil {
		t.Fatal(err)
	}

	// Bob tries to validate with a stale lock_id.
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = s.ValidateAndReleaseLock(ctx, tx, "-100", "00000000-0000-0000-0000-000000000000")
	_ = tx.Rollback()

	if err == nil {
		t.Fatal("expected error when lock held by another user")
	}

	if !strings.Contains(err.Error(), "Alice Smith") {
		t.Errorf("expected error to contain holder name, got: %v", err)
	}
}

func TestValidateAndReleaseLock_ExpiredButMatchingLockID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Insert an already-expired lock directly with a known lock_id.
	knownLockID := "11111111-1111-1111-1111-111111111111"

	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO webapp_locks (chat_id, user_id, user_name, lock_id, expires_at)
		 VALUES ($1, $2, $3, $4, NOW() - INTERVAL '1 minute')`,
		"-100", 42, "Alice Smith", knownLockID,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Validate with the matching lock_id — should succeed (DELETE ignores expires_at).
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = s.ValidateAndReleaseLock(ctx, tx, "-100", knownLockID)
	if err != nil {
		_ = tx.Rollback()

		t.Fatal("expected success for expired but matching lock_id, got:", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseLock(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Acquire a lock.
	lockID, err := s.AcquireLock(ctx, "-100", 42, "Alice Smith")
	if err != nil {
		t.Fatal(err)
	}

	// Release it.
	if err := s.ReleaseLock(ctx, "-100", lockID); err != nil {
		t.Fatal("ReleaseLock failed:", err)
	}

	// Lock should be gone.
	holder, _ := s.GetLockHolder(ctx, "-100")
	if holder != "" {
		t.Errorf("expected lock released, but holder=%q", holder)
	}
}

func TestReleaseLock_WrongLockID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Acquire a lock.
	_, err := s.AcquireLock(ctx, "-100", 42, "Alice Smith")
	if err != nil {
		t.Fatal(err)
	}

	// Try to release with wrong lock_id — should not error but lock remains.
	if err := s.ReleaseLock(ctx, "-100", "wrong-lock-id"); err != nil {
		t.Fatal("ReleaseLock with wrong ID should not error:", err)
	}

	// Lock should still be held.
	holder, _ := s.GetLockHolder(ctx, "-100")
	if holder != "Alice Smith" {
		t.Errorf("expected lock still held by Alice Smith, got %q", holder)
	}
}

func TestListSubscribedMemberIDsForChat(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	g, _ := s.GetOrCreateGroup(ctx, "乃木坂46")
	gen1 := 1
	m1, _ := s.GetOrCreateMember(ctx, g.ID, "MemberA", &gen1, false)
	m2, _ := s.GetOrCreateMember(ctx, g.ID, "MemberB", &gen1, false)
	m3, _ := s.GetOrCreateMember(ctx, g.ID, "MemberC", &gen1, false)

	if _, err := s.AddSubscription(ctx, m1.ID, "-100"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.AddSubscription(ctx, m3.ID, "-100"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.AddSubscription(ctx, m2.ID, "-200"); err != nil { // different chat
		t.Fatal(err)
	}

	ids, err := s.ListSubscribedMemberIDsForChat(ctx, "-100")
	if err != nil {
		t.Fatal(err)
	}

	if len(ids) != 2 {
		t.Fatalf("got %d IDs, want 2", len(ids))
	}

	want := map[int64]bool{m1.ID: true, m3.ID: true}
	for _, id := range ids {
		if !want[id] {
			t.Errorf("unexpected member_id %d", id)
		}
	}
}
