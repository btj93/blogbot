package store

import (
	"context"
	"database/sql"

	"github.com/btj93/blogbot/model"
)

// Querier abstracts Store methods for testability.
type Querier interface {
	GetOrCreateGroup(ctx context.Context, name string) (*model.Group, error)
	GetGroupByName(ctx context.Context, name string) (*model.Group, error)
	GetGroupByID(ctx context.Context, id int64) (*model.Group, error)
	ListGroups(ctx context.Context) ([]model.Group, error)
	GetOrCreateMember(
		ctx context.Context,
		groupID int64,
		name string,
		generation *int,
		disabled bool,
	) (*model.Member, error)
	GetMemberByGroupAndName(ctx context.Context, groupID int64, name string) (*model.Member, error)
	GetMemberByID(ctx context.Context, id int64) (*model.Member, error)
	ListMembersByGroup(ctx context.Context, groupID int64) ([]model.Member, error)
	ListEnabledMembersByGroupAndGeneration(ctx context.Context, groupID int64, generation *int) ([]model.Member, error)
	ListGenerationsForGroup(ctx context.Context, groupID int64) ([]*int, error)
	AddSubscription(ctx context.Context, memberID int64, chatID string) (bool, error)
	RemoveSubscription(ctx context.Context, memberID int64, chatID string) (bool, error)
	AddSubscriptionTx(ctx context.Context, tx *sql.Tx, memberID int64, chatID string) (bool, error)
	RemoveSubscriptionTx(ctx context.Context, tx *sql.Tx, memberID int64, chatID string) (bool, error)
	IsSubscribed(ctx context.Context, memberID int64, chatID string) (bool, error)
	GetSubscriberChatIDs(ctx context.Context, memberID int64) ([]string, error)
	ListSubscribedMemberIDsForChat(ctx context.Context, chatID string) ([]int64, error)
	AddSubscriptionForAllInGeneration(ctx context.Context, groupID int64, generation *int, chatID string) error
	RemoveSubscriptionForAllInGeneration(ctx context.Context, groupID int64, generation *int, chatID string) error
	ClaimBlog(ctx context.Context, url string) (bool, error)
	UpsertShowroomRoom(ctx context.Context, memberID int64, roomID string, url string) error
	UpdateNextLive(ctx context.Context, memberID int64, epoch *int64, text *string) error
	ListShowroomRoomsWithRoomID(ctx context.Context) ([]model.ShowroomRoom, error)
	ListShowroomRoomsWithURL(ctx context.Context) ([]model.ShowroomRoom, error)
	AcquireLock(ctx context.Context, chatID string, userID int64, userName string) (string, error)
	GetLockHolder(ctx context.Context, chatID string) (string, error)
	ReleaseLock(ctx context.Context, chatID string, lockID string) error
	ValidateAndReleaseLock(ctx context.Context, tx *sql.Tx, chatID string, lockID string) error
	BeginTx(ctx context.Context) (*sql.Tx, error)
}
