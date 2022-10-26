package model

import (
	"context"
	"testing"
	"time"

	"github.com/sbasestarter/customer-service-be/config"
	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func Test1(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := NewMongoModel(&config.GetConfig().MongoConfig, nil)
	m1, ok := m.(*mongoModelImpl)
	assert.True(t, ok)

	collectionNames, err := m1.mongoCli.Database(m1.cfg.DB).ListCollectionNames(context.Background(), bson.D{})
	assert.Nil(t, err)

	for _, name := range collectionNames {
		_ = m1.mongoCli.Database(m1.cfg.DB).Collection(name).Drop(context.TODO())
	}

	talkID, err := m.CreateTalk(ctx, &defs.TalkInfoW{
		Status:    defs.TalkStatusOpened,
		Title:     "testTalk1",
		StartAt:   time.Now().Unix(),
		CreatorID: 1,
	})
	assert.Nil(t, err)
	t.Log(talkID)

	err = m.AddTalkMessage(ctx, talkID, &defs.TalkMessageW{
		Type: defs.TalkMessageTypeText,
		Text: "talk_message_1",
	})
	assert.Nil(t, err)

	err = m.AddTalkMessage(ctx, talkID, &defs.TalkMessageW{
		Type: defs.TalkMessageTypeImage,
		Data: []byte("talk_image_1"),
	})
	assert.Nil(t, err)

	err = m.AddTalkMessage(ctx, talkID, &defs.TalkMessageW{
		Type: defs.TalkMessageTypeText,
		Text: "talk_message_3",
	})
	assert.Nil(t, err)

	err = m.AddTalkMessage(ctx, talkID, &defs.TalkMessageW{
		Type: defs.TalkMessageTypeText,
		Text: "talk_message_4",
	})
	assert.Nil(t, err)

	messages, err := m.GetTalkMessages(ctx, talkID, 0, 0)
	assert.Nil(t, err)
	assert.EqualValues(t, 4, len(messages))
	assert.EqualValues(t, "talk_message_1", messages[0].Text)
	assert.EqualValues(t, "talk_message_4", messages[3].Text)

	messages, err = m.GetTalkMessages(ctx, talkID, 0, 1)
	assert.Nil(t, err)
	assert.EqualValues(t, 1, len(messages))
	assert.EqualValues(t, "talk_message_1", messages[0].Text)

	messages, err = m.GetTalkMessages(ctx, talkID, 1, 2)
	assert.Nil(t, err)
	assert.EqualValues(t, 2, len(messages))
	assert.EqualValues(t, "talk_message_3", messages[1].Text)

	err = m.CloseTalk(ctx, talkID)
	assert.Nil(t, err)

	err = m.OpenTalk(ctx, talkID)
	assert.Nil(t, err)

	err = m.OpenTalk(ctx, talkID[0:len(talkID)-1]+"9")
	assert.NotNil(t, err)

	talks, err := m.QueryTalks(ctx, 1, 0, "", nil)
	assert.Nil(t, err)
	assert.EqualValues(t, 1, len(talks))
	assert.EqualValues(t, defs.TalkStatusOpened, talks[0].Status)

	talks, err = m.QueryTalks(ctx, 1, 0, "", []defs.TalkStatus{defs.TalkStatusOpened})
	assert.Nil(t, err)
	assert.EqualValues(t, 1, len(talks))

	talks, err = m.QueryTalks(ctx, 1, 0, "", []defs.TalkStatus{defs.TalkStatusClosed})
	assert.Nil(t, err)
	assert.EqualValues(t, 0, len(talks))
}
