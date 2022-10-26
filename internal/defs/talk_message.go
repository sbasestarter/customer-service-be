package defs

type TalkMessageType int

const (
	TalkMessageTypeUnknown TalkMessageType = iota
	TalkMessageTypeText
	TalkMessageTypeImage
)

type TalkMessageW struct {
	At              int64           `bson:"At"`
	CustomerMessage bool            `bson:"CustomerMessage"`
	Type            TalkMessageType `bson:"Type"`
	SenderID        uint64          `bson:"SenderID"`
	Text            string          `bson:"Text,omitempty"`
	Data            []byte          `bson:"Data,omitempty"`
}

type TalkMessageR struct {
	MessageID    string `bson:"_id"`
	TalkMessageW `bson:"inline"`
}
