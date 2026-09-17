package billing

import (
	"testing"

	"github.com/example/evcharging/internal/domain"
)

func TestTieredCalculatorDCTariff(t *testing.T) {
	tariff := dcTariff()
	tests := []struct {
		name      string
		energyKWh float64
		wantPaise int64
	}{
		{"zero energy still pays minimum", 0, 15000},
		{"below minimum", 5, 15000},
		{"first tier boundary", 10, 20000},
		{"one unit in second tier", 11, 21400},
		{"second tier boundary", 25, 41000},
		{"one unit in final tier", 26, 41900},
	}
	calculator := TieredCalculator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, err := calculator.Calculate(tariff, tt.energyKWh, 0)
			if err != nil {
				t.Fatalf("Calculate() error = %v", err)
			}
			if price.FinalPaise != tt.wantPaise {
				t.Errorf("FinalPaise = %d, want %d", price.FinalPaise, tt.wantPaise)
			}
		})
	}
}

func TestTieredCalculatorConnectorSpecificTariffs(t *testing.T) {
	calculator := TieredCalculator{}
	acPrice, err := calculator.Calculate(acTariff(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	dcPrice, err := calculator.Calculate(dcTariff(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if acPrice.FinalPaise != 12000 || dcPrice.FinalPaise != 20000 {
		t.Fatalf("AC/DC prices = %d/%d, want 12000/20000", acPrice.FinalPaise, dcPrice.FinalPaise)
	}
}

func TestTieredCalculatorAppliesPromoAfterMinimumCharge(t *testing.T) {
	price, err := (TieredCalculator{}).Calculate(dcTariff(), 5, 20)
	if err != nil {
		t.Fatal(err)
	}
	if price.BasePaise != 15000 || price.DiscountPaise != 3000 || price.FinalPaise != 12000 {
		t.Fatalf("price = %+v, want base=15000 discount=3000 final=12000", price)
	}
}

func TestTieredCalculatorRejectsNegativeEnergy(t *testing.T) {
	if _, err := (TieredCalculator{}).Calculate(dcTariff(), -1, 0); err == nil {
		t.Fatal("expected an error for negative energy")
	}
}

func dcTariff() domain.Tariff {
	return domain.Tariff{
		ConnectorType: domain.ConnectorDC, MinimumChargePaise: 15000,
		Tiers: []domain.TariffTier{
			{UpToKWh: floatPointer(10), RatePaisePerKWh: 2000},
			{UpToKWh: floatPointer(25), RatePaisePerKWh: 1400},
			{UpToKWh: nil, RatePaisePerKWh: 900},
		},
	}
}

func acTariff() domain.Tariff {
	return domain.Tariff{
		ConnectorType: domain.ConnectorAC, MinimumChargePaise: 10000,
		Tiers: []domain.TariffTier{
			{UpToKWh: floatPointer(10), RatePaisePerKWh: 1200},
			{UpToKWh: floatPointer(25), RatePaisePerKWh: 900},
			{UpToKWh: nil, RatePaisePerKWh: 700},
		},
	}
}

func floatPointer(value float64) *float64 { return &value }
