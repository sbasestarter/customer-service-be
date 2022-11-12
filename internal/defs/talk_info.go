package defs

type TalkStatus int

const (
	TalkStatusNone TalkStatus = iota
	TalkStatusOpened
	TalkStatusClosed
)

type TalkInfoW struct {
	Status          TalkStatus `bson:"Status"`
	Title           string     `bson:"Title"`
	StartAt         int64      `bson:"StartAt"`
	FinishedAt      int64      `bson:"FinishedAt"`
	CreatorID       uint64     `bson:"CreatorID"`
	ServiceID       uint64     `bson:"ServiceID"`
	CreatorUserName string     `bson:"CreatorUserName"`
}

type TalkInfoR struct {
	TalkID    string `bson:"_id"`
	TalkInfoW `bson:"inline"`
}
