package handler

import (
	"testing"

	"github.com/CriarBrand/qianchuan-backend/internal/sdk"
)

func TestProtectedMaterialsBySpendSortsLocally(t *testing.T) {
	materials := []sdk.UniPromotionMaterialInfo{
		{MaterialId: 10, StatCost: 12},
		{MaterialId: 20, StatCost: 80},
		{MaterialId: 30, StatCost: 35},
		{MaterialId: 40, StatCost: 60},
		{MaterialId: 50, StatCost: 0},
	}

	protected := protectedMaterialsBySpend(materials)
	for _, id := range []uint64{20, 30, 40} {
		if !protected[id] {
			t.Fatalf("expected material %d to be protected: %#v", id, protected)
		}
	}
	for _, id := range []uint64{10, 50} {
		if protected[id] {
			t.Fatalf("material %d must not be protected: %#v", id, protected)
		}
	}
}

func TestProtectedMaterialsBySpendUsesMaxDuplicateCost(t *testing.T) {
	materials := []sdk.UniPromotionMaterialInfo{
		{MaterialId: 1, StatCost: 5},
		{MaterialId: 2, StatCost: 70},
		{MaterialId: 3, StatCost: 40},
		{MaterialId: 1, StatCost: 90},
		{MaterialId: 4, StatCost: 50},
	}

	protected := protectedMaterialsBySpend(materials)
	for _, id := range []uint64{1, 2, 4} {
		if !protected[id] {
			t.Fatalf("expected material %d to be protected: %#v", id, protected)
		}
	}
	if protected[3] {
		t.Fatalf("material 3 must not be protected: %#v", protected)
	}
}
