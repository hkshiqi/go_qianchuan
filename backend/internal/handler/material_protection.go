package handler

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
)

// topSpendingMaterialIDs returns the three material IDs with the highest
// server-verified spending for a plan. The ordering is enforced locally
// because remote material APIs do not guarantee a stable sort order.
func topSpendingMaterialIDs(
	ctx context.Context,
	client sdk.QianchuanClient,
	accessToken string,
	advertiserID uint64,
	adID uint64,
	materialType string,
	startDate string,
	endDate string,
) (map[uint64]bool, error) {
	if startDate == "" {
		startDate = time.Now().Format("2006-01-02")
	}
	if endDate == "" {
		endDate = startDate
	}
	if materialType == "" {
		materialType = "VIDEO"
	}
	materialType = strings.ToUpper(materialType)

	// VIDEO/TITLE support material_status=ALL. Omitting the filter for other
	// material types avoids sending an unsupported enum while still checking
	// every active item of that type.
	materialStatus := ""
	if materialType == "VIDEO" || materialType == "TITLE" {
		materialStatus = "ALL"
	}

	materials := make([]sdk.UniPromotionMaterialInfo, 0, 100)
	for page := 1; page <= 100; page++ {
		resp, err := client.UniPromotionMaterialGet(
			ctx,
			sdk.UniPromotionMaterialGetReq{
				AccessToken:        accessToken,
				AdvertiserId:       advertiserID,
				AdId:               adID,
				MaterialType:       materialType,
				Status:             materialStatus,
				MaterialSelectType: "ALL",
				StartDate:          startDate,
				EndDate:            endDate,
				Fields:             []string{"stat_cost_for_roi2"},
				OrderField:         "stat_cost_for_roi2",
				OrderType:          "DESC",
				Page:               page,
				PageSize:           100,
			},
		)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, fmt.Errorf("素材接口返回空响应")
		}
		if resp.Code != 0 {
			return nil, fmt.Errorf("素材接口返回[%d]: %s", resp.Code, resp.Message)
		}
		materials = append(materials, resp.Data.Materials...)
		if len(resp.Data.Materials) == 0 ||
			len(resp.Data.Materials) < 100 ||
			resp.Data.PageInfo.TotalPage <= page {
			break
		}
	}
	return protectedMaterialsBySpend(materials), nil
}

func protectedMaterialsBySpend(materials []sdk.UniPromotionMaterialInfo) map[uint64]bool {
	spendByMaterial := make(map[uint64]float64, len(materials))
	for _, material := range materials {
		if material.MaterialId == 0 || material.StatCost <= 0 {
			continue
		}
		if material.StatCost > spendByMaterial[material.MaterialId] {
			spendByMaterial[material.MaterialId] = material.StatCost
		}
	}

	type materialSpend struct {
		id    uint64
		spend float64
	}
	ranked := make([]materialSpend, 0, len(spendByMaterial))
	for id, spend := range spendByMaterial {
		ranked = append(ranked, materialSpend{id: id, spend: spend})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].spend == ranked[j].spend {
			return ranked[i].id > ranked[j].id
		}
		return ranked[i].spend > ranked[j].spend
	})

	protected := make(map[uint64]bool, 3)
	for i := 0; i < len(ranked) && i < 3; i++ {
		protected[ranked[i].id] = true
	}
	return protected
}
