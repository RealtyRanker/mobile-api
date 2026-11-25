package scoring

import (
	"testing"

	"github.com/asmisnik/mobile-api/internal/model"
)

func baseFlat() *model.Flat {
	return &model.Flat{
		DealType:   "rent",
		Price:      80000,
		TotalArea:  45,
		RoomNumber: 1,
	}
}

func TestScoreDecreasesWithPrice(t *testing.T) {
	cheap := baseFlat()
	expensive := baseFlat()
	expensive.Price = 200000

	if Score(cheap, nil) <= Score(expensive, nil) {
		t.Fatalf("expected cheaper flat to score higher: cheap=%d expensive=%d",
			Score(cheap, nil), Score(expensive, nil))
	}
}

func TestScoreIncreasesWithArea(t *testing.T) {
	small := baseFlat()
	big := baseFlat()
	big.TotalArea = 90

	if Score(big, nil) <= Score(small, nil) {
		t.Fatalf("expected bigger flat to score higher: small=%d big=%d",
			Score(small, nil), Score(big, nil))
	}
}

func TestCustomParamsOverrideDefault(t *testing.T) {
	f := baseFlat()
	defaultScore := Score(f, nil)

	custom := &CustomParams{AllArea: 3000} // far above the default 300/sqm
	customScore := Score(f, custom)

	if customScore <= defaultScore {
		t.Fatalf("expected a much higher AllArea multiplier to raise the score: default=%d custom=%d",
			defaultScore, customScore)
	}
}

func TestSaleScoresAreScaledUp(t *testing.T) {
	rent := baseFlat()
	sale := baseFlat()
	sale.DealType = "sale"
	sale.Price = 15000000

	// Sale scoring uses a 200x scale on most constants, so a sale score
	// should land in a completely different order of magnitude than rent.
	if Score(sale, nil) <= Score(rent, nil)*50 {
		t.Fatalf("expected sale score to be scaled far above rent score: rent=%d sale=%d",
			Score(rent, nil), Score(sale, nil))
	}
}
