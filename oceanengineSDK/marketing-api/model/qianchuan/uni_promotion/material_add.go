package unipromotion

import "github.com/bububa/oceanengine/marketing-api/util"

// MaterialAddRequest adds existing material-library entries to a plan.
type MaterialAddRequest struct {
	AdvertiserID                  uint64                      `json:"advertiser_id"`
	AdID                          uint64                      `json:"ad_id"`
	ProgrammaticCreativeMediaList []ProgrammaticCreativeMedia `json:"programmatic_creative_media_list"`
}

// Encode implements PostRequest.
func (r MaterialAddRequest) Encode() []byte {
	return util.JSONMarshal(r)
}
