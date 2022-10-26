package vo

import (
	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
)

func TaskStatusMapPb2Db(status customertalkpb.TalkStatus) defs.TalkStatus {
	switch status {
	case customertalkpb.TalkStatus_TALK_STATUS_OPENED:
		return defs.TalkStatusOpened
	case customertalkpb.TalkStatus_TALK_STATUS_CLOSED:
		return defs.TalkStatusClosed
	default:
		return defs.TalkStatusNone
	}
}

func TaskStatusesMapPb2Db(statuses []customertalkpb.TalkStatus) []defs.TalkStatus {
	if statuses == nil {
		return nil
	}

	rStatues := make([]defs.TalkStatus, 0, len(statuses))

	for _, talkStatus := range statuses {
		rStatues = append(rStatues, TaskStatusMapPb2Db(talkStatus))
	}

	return rStatues
}

func TaskStatusMapDB2Pb(status defs.TalkStatus) customertalkpb.TalkStatus {
	switch status {
	case defs.TalkStatusOpened:
		return customertalkpb.TalkStatus_TALK_STATUS_OPENED
	case defs.TalkStatusClosed:
		return customertalkpb.TalkStatus_TALK_STATUS_CLOSED
	default:
		return customertalkpb.TalkStatus_TALK_STATUS_UNSPECIFIED
	}
}

func TalkInfoRDb2Pb(talkInfo *defs.TalkInfoR) *customertalkpb.TalkInfo {
	if talkInfo == nil {
		return nil
	}

	return &customertalkpb.TalkInfo{
		TalkId:     talkInfo.TalkID,
		Status:     TaskStatusMapDB2Pb(talkInfo.Status),
		Title:      talkInfo.Title,
		StartedAt:  uint64(talkInfo.StartAt),
		FinishedAt: uint64(talkInfo.FinishedAt),
	}
}

func TalkInfoRsDB2Pb(talkInfos []*defs.TalkInfoR) []*customertalkpb.TalkInfo {
	if talkInfos == nil {
		return nil
	}

	rTalkInfos := make([]*customertalkpb.TalkInfo, 0, len(talkInfos))

	for _, info := range talkInfos {
		rTalkInfos = append(rTalkInfos, TalkInfoRDb2Pb(info))
	}

	return rTalkInfos
}

func TalkMessageWPb2Db(message *customertalkpb.TalkMessageW) *defs.TalkMessageW {
	if message == nil {
		return nil
	}

	dbMessage := &defs.TalkMessageW{}

	if message.GetText() != "" {
		dbMessage.Type = defs.TalkMessageTypeText
		dbMessage.Text = message.GetText()
	} else if len(message.GetImage()) > 0 {
		dbMessage.Type = defs.TalkMessageTypeImage
		dbMessage.Data = message.GetImage()
	} else {
		dbMessage.Type = defs.TalkMessageTypeUnknown
	}

	return dbMessage
}

func TalkMessageDB2Pb(message *defs.TalkMessageW) *customertalkpb.TalkMessage {
	if message == nil {
		return nil
	}

	pbMessage := &customertalkpb.TalkMessage{
		At:              uint64(message.At),
		CustomerMessage: message.CustomerMessage,
	}

	switch message.Type {
	case defs.TalkMessageTypeText:
		pbMessage.Message = &customertalkpb.TalkMessage_Text{
			Text: message.Text,
		}
	case defs.TalkMessageTypeImage:
		pbMessage.Message = &customertalkpb.TalkMessage_Image{
			Image: message.Data,
		}
	}

	return pbMessage
}

func TalkMessagesRDb2Pb(messages []*defs.TalkMessageR) []*customertalkpb.TalkMessage {
	if messages == nil {
		return nil
	}

	pbMessages := make([]*customertalkpb.TalkMessage, 0, len(messages))

	for _, message := range messages {
		pbMessages = append(pbMessages, TalkMessageDB2Pb(&message.TalkMessageW))
	}

	return pbMessages
}
