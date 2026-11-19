package unipromotion

import (
	"context"

	"github.com/bububa/oceanengine/marketing-api/core"
	unipromotion "github.com/bububa/oceanengine/marketing-api/model/qianchuan/uni_promotion"
)

// MaterialAdd adds existing material-library entries to a plan.
func MaterialAdd(ctx context.Context, clt *core.SDKClient, accessToken string, req *unipromotion.MaterialAddRequest) error {
	return clt.Post(ctx, "v1.0/qianchuan/uni_promotion/ad/material/add/", req, nil, accessToken)
}
