package model

import "time"

type Group struct {
	ID        int64
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Member struct {
	ID         int64
	GroupID    int64
	Name       string
	Generation *int // nil = unknown
	Disabled   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Subscription struct {
	ID        int64
	MemberID  int64
	ChatID    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type BlogProgress struct {
	ID        int64
	URL       string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ShowroomRoom struct {
	ID            int64
	MemberID      int64
	RoomID        string
	URL           string
	NextLiveEpoch *int64
	NextLiveText  *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Blog struct {
	Title     string
	Name      string // member name
	URL       string
	ImageURLs []string
}
