package billing

import (
	"fmt"
	"math"
	"sort"

	"github.com/example/evcharging/internal/domain"
)

type Price struct {
	BasePaise     int64 `json:"base_paise"`
	DiscountPaise int64 `json:"discount_paise"`
	FinalPaise    int64 `json:"final_paise"`
}

type Calculator interface {
	Calculate(tariff domain.Tariff, energyKWh float64, discountPercent int) (Price, error)
}

type TieredCalculator struct{}

func (TieredCalculator) Calculate(tariff domain.Tariff, energyKWh float64, discountPercent int) (Price, error) {
	if energyKWh < 0 || math.IsNaN(energyKWh) || math.IsInf(energyKWh, 0) {
		return Price{}, fmt.Errorf("%w: energy must be a finite non-negative number", domain.ErrInvalidInput)
	}
	if discountPercent < 0 || discountPercent > 100 {
		return Price{}, fmt.Errorf("%w: discount must be between 0 and 100", domain.ErrInvalidInput)
	}
	if len(tariff.Tiers) == 0 {
		return Price{}, fmt.Errorf("%w: tariff has no tiers", domain.ErrInvalidInput)
	}

	tiers := append([]domain.TariffTier(nil), tariff.Tiers...)
	sort.SliceStable(tiers, func(i, j int) bool {
		if tiers[i].UpToKWh == nil {
			return false
		}
		if tiers[j].UpToKWh == nil {
			return true
		}
		return *tiers[i].UpToKWh < *tiers[j].UpToKWh
	})

	remaining, lowerBound, rawPaise := energyKWh, 0.0, 0.0
	for _, tier := range tiers {
		if remaining <= 0 {
			break
		}
		quantity := remaining
		if tier.UpToKWh != nil {
			if *tier.UpToKWh <= lowerBound {
				return Price{}, fmt.Errorf("%w: tariff tier bounds must increase", domain.ErrInvalidInput)
			}
			quantity = math.Min(remaining, *tier.UpToKWh-lowerBound)
			lowerBound = *tier.UpToKWh
		}
		rawPaise += quantity * float64(tier.RatePaisePerKWh)
		remaining -= quantity
	}
	if remaining > 0 {
		return Price{}, fmt.Errorf("%w: tariff does not cover all energy", domain.ErrInvalidInput)
	}

	base := max(int64(math.Round(rawPaise)), tariff.MinimumChargePaise)
	discount := int64(math.Round(float64(base) * float64(discountPercent) / 100))
	return Price{BasePaise: base, DiscountPaise: discount, FinalPaise: base - discount}, nil
}
