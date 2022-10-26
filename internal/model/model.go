package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/sbasestarter/customer-service-be/config"
	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sgostarter/i/l"
	"github.com/sgostarter/libeasygo/commerr"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	collectionTalkInfo     = "talk_info"
	collectionTalkTemplate = "talk:%s"
)

func NewMongoModel(cfg *config.MongoConfig, logger l.Wrapper) defs.Model {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	if cfg == nil {
		logger.Fatal("NoCfgOnCreateModel")

		return nil
	}

	mongoServer := cfg.Server
	if !strings.HasPrefix(mongoServer, "mongodb://") {
		mongoServer = "mongodb://" + mongoServer
	}

	clientOptions := options.Client().ApplyURI(mongoServer).SetAuth(options.Credential{
		AuthSource: cfg.DB,
		Username:   cfg.UserName,
		Password:   cfg.Password,
	})

	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Fatal("MongoConnect")
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Fatal("MongoPing")
	}

	return &mongoModelImpl{
		cfg:      cfg,
		mongoCli: client,
	}
}

type mongoModelImpl struct {
	cfg      *config.MongoConfig
	mongoCli *mongo.Client
}

func (m *mongoModelImpl) CreateTalk(ctx context.Context, talkInfo *defs.TalkInfoW) (talkID string, err error) {
	r, err := m.mongoCli.Database(m.cfg.DB).Collection(collectionTalkInfo).InsertOne(ctx, talkInfo)
	if err != nil {
		return
	}

	if oid, ok := r.InsertedID.(primitive.ObjectID); ok {
		talkID = oid.Hex()
	}

	return
}

func (m *mongoModelImpl) OpenTalk(ctx context.Context, talkID string) (err error) {
	return m.updateTalkInfo(ctx, talkID, bson.M{
		"Status": defs.TalkStatusOpened,
	})
}

func (m *mongoModelImpl) CloseTalk(ctx context.Context, talkID string) (err error) {
	return m.updateTalkInfo(ctx, talkID, bson.M{
		"Status": defs.TalkStatusClosed,
	})
}

func (m *mongoModelImpl) AddTalkMessage(ctx context.Context, talkID string, message *defs.TalkMessageW) (err error) {
	_, err = m.mongoCli.Database(m.cfg.DB).Collection(m.talkCollectionKey(talkID)).InsertOne(ctx, message)

	return
}

func (m *mongoModelImpl) GetTalkMessages(ctx context.Context, talkID string, offset, count int64) (messages []*defs.TalkMessageR, err error) {
	findOptions := options.Find()
	if count > 0 {
		findOptions.SetSkip(offset)
		findOptions.SetLimit(count)
	}

	cursor, err := m.mongoCli.Database(m.cfg.DB).Collection(m.talkCollectionKey(talkID)).Find(ctx, bson.D{}, findOptions)
	if err != nil {
		return
	}

	err = cursor.All(ctx, &messages)

	return
}

func (m *mongoModelImpl) QueryTalks(ctx context.Context, creatorID, serviceID uint64, talkID string,
	statuses []defs.TalkStatus) (talks []*defs.TalkInfoR, err error) {
	return m.queryTalksEx(ctx, creatorID, serviceID, talkID, statuses, nil)
}

func (m *mongoModelImpl) GetPendingTalkInfos(ctx context.Context) ([]*defs.TalkInfoR, error) {
	bsonM := bson.M{}
	bsonM["ServiceID"] = 0

	talkInfos, err := m.queryTalksEx(ctx, 0, 0, "", []defs.TalkStatus{defs.TalkStatusOpened}, bsonM)
	if err != nil {
		return nil, err
	}

	return talkInfos, nil
}

func (m *mongoModelImpl) UpdateTalkServiceID(ctx context.Context, talkID string, serviceID uint64) (err error) {
	return m.updateTalkInfo(ctx, talkID, bson.M{
		"ServiceID": serviceID,
	})
}

//
//
//

func (m *mongoModelImpl) queryTalkFilter(creatorID, serviceID uint64, talkID string, statuses []defs.TalkStatus) (filter bson.M, err error) {
	filter = bson.M{}
	if creatorID > 0 {
		filter["CreatorID"] = creatorID
	}

	if serviceID > 0 {
		filter["ServiceID"] = serviceID
	}

	if len(statuses) > 0 {
		filter["Status"] = bson.M{"$in": statuses}
	}

	if talkID != "" {
		var objectID primitive.ObjectID

		objectID, err = primitive.ObjectIDFromHex(talkID)
		if err != nil {
			err = commerr.ErrInvalidArgument

			return
		}

		filter["_id"] = objectID
	}

	return
}

func (m *mongoModelImpl) queryTalksEx(ctx context.Context, creatorID, serviceID uint64, talkID string,
	statuses []defs.TalkStatus, bsonM bson.M) (talks []*defs.TalkInfoR, err error) {
	collection := m.mongoCli.Database(m.cfg.DB).Collection(collectionTalkInfo)

	filter, err := m.queryTalkFilter(creatorID, serviceID, talkID, statuses)
	if err != nil {
		err = commerr.ErrInvalidArgument

		return
	}

	for k, v := range bsonM {
		filter[k] = v
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return
	}

	err = cursor.All(ctx, &talks)

	return
}

func (m *mongoModelImpl) updateTalkInfo(ctx context.Context, talkID string, updateMap bson.M) (err error) {
	objectID, err := primitive.ObjectIDFromHex(talkID)
	if err != nil {
		return
	}

	r := m.mongoCli.Database(m.cfg.DB).Collection(collectionTalkInfo).FindOneAndUpdate(ctx,
		bson.M{
			"_id": objectID,
		}, bson.M{
			"$set": updateMap,
		})

	err = r.Err()

	return
}

func (m *mongoModelImpl) talkCollectionKey(talkID string) string {
	return fmt.Sprintf(collectionTalkTemplate, talkID)
}
