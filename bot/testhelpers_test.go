package bot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/btj93/blogbot/model"
	"github.com/btj93/blogbot/telegram"
)

// Compile-time interface checks.
var _ telegram.Sender = (*fakeSender)(nil)

type fakeSender struct {
	sentTexts       []fakeSentText
	sentMedia       []fakeSentMedia
	loggedTexts     []string
	edits           []fakeEdit
	adminMap        map[int64][]tgbotapi.ChatMember
	logChatID       int64
	callbackAnswers []fakeCallbackAnswer
	menuButtonURL   string
	menuButtonReset bool
}

type fakeCallbackAnswer struct {
	CallbackQueryID string
	Text            string
}

type fakeSentText struct {
	ChatID int64
	Text   string
	Opts   *telegram.SendTextOpts
}

type fakeSentMedia struct {
	ChatID int64
	Photos [][]byte
}

type fakeEdit struct {
	ChatID    int64
	MessageID int
	Text      string
	Markup    tgbotapi.InlineKeyboardMarkup
}

func (f *fakeSender) SendText(_ context.Context, chatID int64, text string, opts *telegram.SendTextOpts) error {
	f.sentTexts = append(f.sentTexts, fakeSentText{chatID, text, opts})
	return nil
}

func (f *fakeSender) SendMediaGroup(_ context.Context, chatID int64, photos [][]byte) error {
	f.sentMedia = append(f.sentMedia, fakeSentMedia{chatID, photos})
	return nil
}

func (f *fakeSender) LogText(_ context.Context, text string) {
	f.loggedTexts = append(f.loggedTexts, text)
}

func (f *fakeSender) LogChatID() int64 { return f.logChatID }

func (f *fakeSender) EditMessageTextAndMarkup(
	_ context.Context,
	chatID int64,
	messageID int,
	text string,
	markup tgbotapi.InlineKeyboardMarkup,
) error {
	f.edits = append(f.edits, fakeEdit{chatID, messageID, text, markup})
	return nil
}

func (f *fakeSender) GetChatAdministrators(_ context.Context, chatID int64) ([]tgbotapi.ChatMember, error) {
	if f.adminMap != nil {
		return f.adminMap[chatID], nil
	}

	return nil, nil
}

func (f *fakeSender) GetChat(_ context.Context, chatID int64) (tgbotapi.Chat, error) {
	return tgbotapi.Chat{ID: chatID, Title: "Test Chat"}, nil
}

func (f *fakeSender) AnswerCallbackQuery(_ context.Context, callbackQueryID, text string) error {
	f.callbackAnswers = append(f.callbackAnswers, fakeCallbackAnswer{callbackQueryID, text})
	return nil
}

func (f *fakeSender) SetChatMenuButton(_ context.Context, url, text string) error {
	f.menuButtonURL = url
	return nil
}

func (f *fakeSender) ResetChatMenuButton(_ context.Context) error {
	f.menuButtonReset = true
	return nil
}

// fakeQuerier implements store.Querier with canned data.
type fakeQuerier struct {
	groups      []model.Group
	generations []*int
	members     []model.Member
	memberByID  map[int64]*model.Member
	subscribed  map[int64]map[string]bool // memberID -> chatID -> subscribed
	locks       map[string]*fakeLock      // chatID -> lock
}

type fakeLock struct {
	userID   int64
	userName string
	lockID   string
}

func newFakeQuerier() *fakeQuerier {
	return &fakeQuerier{
		memberByID: make(map[int64]*model.Member),
		subscribed: make(map[int64]map[string]bool),
		locks:      make(map[string]*fakeLock),
	}
}

// Groups

func (f *fakeQuerier) GetOrCreateGroup(_ context.Context, name string) (*model.Group, error) {
	for i := range f.groups {
		if f.groups[i].Name == name {
			return &f.groups[i], nil
		}
	}

	return &model.Group{Name: name}, nil
}

func (f *fakeQuerier) GetGroupByName(_ context.Context, name string) (*model.Group, error) {
	for i := range f.groups {
		if f.groups[i].Name == name {
			return &f.groups[i], nil
		}
	}

	return nil, sql.ErrNoRows
}

func (f *fakeQuerier) GetGroupByID(_ context.Context, id int64) (*model.Group, error) {
	for i := range f.groups {
		if f.groups[i].ID == id {
			return &f.groups[i], nil
		}
	}

	return nil, sql.ErrNoRows
}

func (f *fakeQuerier) ListGroups(_ context.Context) ([]model.Group, error) {
	return f.groups, nil
}

// Members

func (f *fakeQuerier) GetOrCreateMember(
	_ context.Context,
	groupID int64,
	name string,
	generation *int,
	disabled bool,
) (*model.Member, error) {
	return &model.Member{GroupID: groupID, Name: name, Generation: generation, Disabled: disabled}, nil
}

func (f *fakeQuerier) GetMemberByGroupAndName(_ context.Context, groupID int64, name string) (*model.Member, error) {
	for i := range f.members {
		if f.members[i].GroupID == groupID && f.members[i].Name == name {
			return &f.members[i], nil
		}
	}

	return nil, sql.ErrNoRows
}

func (f *fakeQuerier) GetMemberByID(_ context.Context, id int64) (*model.Member, error) {
	if m, ok := f.memberByID[id]; ok {
		return m, nil
	}

	return nil, sql.ErrNoRows
}

func (f *fakeQuerier) ListMembersByGroup(_ context.Context, groupID int64) ([]model.Member, error) {
	var result []model.Member

	for _, m := range f.members {
		if m.GroupID == groupID {
			result = append(result, m)
		}
	}

	return result, nil
}

func (f *fakeQuerier) ListEnabledMembersByGroupAndGeneration(
	_ context.Context,
	groupID int64,
	generation *int,
) ([]model.Member, error) {
	return f.members, nil
}

func (f *fakeQuerier) ListGenerationsForGroup(_ context.Context, groupID int64) ([]*int, error) {
	return f.generations, nil
}

// Subscriptions

func (f *fakeQuerier) AddSubscription(_ context.Context, memberID int64, chatID string) (bool, error) {
	if f.subscribed[memberID] == nil {
		f.subscribed[memberID] = make(map[string]bool)
	}

	f.subscribed[memberID][chatID] = true

	return true, nil
}

func (f *fakeQuerier) RemoveSubscription(_ context.Context, memberID int64, chatID string) (bool, error) {
	if f.subscribed[memberID] != nil {
		delete(f.subscribed[memberID], chatID)
	}

	return true, nil
}

func (f *fakeQuerier) IsSubscribed(_ context.Context, memberID int64, chatID string) (bool, error) {
	if f.subscribed[memberID] != nil {
		return f.subscribed[memberID][chatID], nil
	}

	return false, nil
}

func (f *fakeQuerier) GetSubscriberChatIDs(_ context.Context, memberID int64) ([]string, error) {
	return nil, nil
}

func (f *fakeQuerier) AddSubscriptionForAllInGeneration(
	_ context.Context,
	groupID int64,
	generation *int,
	chatID string,
) error {
	return nil
}

func (f *fakeQuerier) RemoveSubscriptionForAllInGeneration(
	_ context.Context,
	groupID int64,
	generation *int,
	chatID string,
) error {
	return nil
}

func (f *fakeQuerier) ListSubscribedMemberIDsForChat(_ context.Context, chatID string) ([]int64, error) {
	var ids []int64

	for memberID, chats := range f.subscribed {
		if chats[chatID] {
			ids = append(ids, memberID)
		}
	}

	return ids, nil
}

// Blog Progress

func (f *fakeQuerier) ClaimBlog(_ context.Context, url string) (bool, error) {
	return true, nil
}

// Showroom

func (f *fakeQuerier) UpsertShowroomRoom(_ context.Context, memberID int64, roomID string, url string) error {
	return nil
}

func (f *fakeQuerier) UpdateNextLive(_ context.Context, memberID int64, epoch *int64, text *string) error {
	return nil
}

func (f *fakeQuerier) ListShowroomRoomsWithRoomID(_ context.Context) ([]model.ShowroomRoom, error) {
	return nil, nil
}

func (f *fakeQuerier) ListShowroomRoomsWithURL(_ context.Context) ([]model.ShowroomRoom, error) {
	return nil, nil
}

// Lock methods

func (f *fakeQuerier) AcquireLock(_ context.Context, chatID string, userID int64, userName string) (string, error) {
	if existing, ok := f.locks[chatID]; ok {
		if existing.userID == userID {
			return existing.lockID, nil
		}

		return "", nil
	}

	lockID := "fake-lock-id"
	f.locks[chatID] = &fakeLock{userID: userID, userName: userName, lockID: lockID}

	return lockID, nil
}

func (f *fakeQuerier) GetLockHolder(_ context.Context, chatID string) (string, error) {
	if l, ok := f.locks[chatID]; ok {
		return l.userName, nil
	}

	return "", nil
}

func (f *fakeQuerier) ReleaseLock(_ context.Context, chatID string, lockID string) error {
	if l, ok := f.locks[chatID]; ok && l.lockID == lockID {
		delete(f.locks, chatID)
	}

	return nil
}

func (f *fakeQuerier) ValidateAndReleaseLock(_ context.Context, _ *sql.Tx, chatID string, lockID string) error {
	if l, ok := f.locks[chatID]; ok {
		if l.lockID == lockID {
			delete(f.locks, chatID)
			return nil
		}

		return fmt.Errorf("locked by %s", l.userName)
	}

	return errors.New("invalid lock")
}

func (f *fakeQuerier) BeginTx(_ context.Context) (*sql.Tx, error) {
	return nil, nil
}

func (f *fakeQuerier) AddSubscriptionTx(_ context.Context, _ *sql.Tx, memberID int64, chatID string) (bool, error) {
	return f.AddSubscription(context.Background(), memberID, chatID)
}

func (f *fakeQuerier) RemoveSubscriptionTx(_ context.Context, _ *sql.Tx, memberID int64, chatID string) (bool, error) {
	return f.RemoveSubscription(context.Background(), memberID, chatID)
}
