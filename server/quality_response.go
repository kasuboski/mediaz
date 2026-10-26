package server

import (
	"context"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

func (s Server) profileDTO(ctx context.Context, profile sqlcdb.QualityProfile) (qualityProfileResponse, error) {
	rows, err := s.manager.ListQualityProfileRows(ctx, profile.ID)
	if err != nil {
		return qualityProfileResponse{}, err
	}
	return qualityProfileDTO(profile, rows), nil
}
func (s Server) profilesDTO(ctx context.Context, profiles []sqlcdb.QualityProfile) ([]qualityProfileResponse, error) {
	rows, err := s.manager.ListQualityProfileRows(ctx, 0)
	if err != nil {
		return nil, err
	}
	if profiles == nil {
		return nil, nil
	}
	// Preserve the historical presentation order: profiles first encountered in
	// the quality rows (descending minimum size), then profiles without items.
	byID := make(map[int64]sqlcdb.QualityProfile, len(profiles))
	for _, p := range profiles {
		byID[p.ID] = p
	}
	result := make([]qualityProfileResponse, 0, len(profiles))
	for _, row := range rows {
		if p, ok := byID[row.QualityProfile.ID]; ok {
			result = append(result, qualityProfileDTO(p, rows))
			delete(byID, p.ID)
		}
	}
	for _, p := range profiles {
		if _, ok := byID[p.ID]; ok {
			result = append(result, qualityProfileDTO(p, rows))
		}
	}
	return result, nil
}
