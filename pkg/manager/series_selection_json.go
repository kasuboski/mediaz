package manager

import (
	"encoding/json"
	"fmt"
)

func (p *AddSeriesRequest) UnmarshalJSON(data []byte) error {
	type plain AddSeriesRequest
	if err := rejectNull(data, "tmdbID", "qualityProfileID", "monitored", "monitorNewSeasons", "selection"); err != nil {
		return err
	}
	return json.Unmarshal(data, (*plain)(p))
}
func (p *SeriesAddSelection) UnmarshalJSON(data []byte) error {
	type plain SeriesAddSelection
	if err := rejectNull(data, "mode", "seasons"); err != nil {
		return err
	}
	return json.Unmarshal(data, (*plain)(p))
}
func requireMonitored(data []byte) error {
	if err := rejectNull(data, "monitored"); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if _, ok := fields["monitored"]; !ok {
		return fmt.Errorf("%w: monitored is required", ErrValidation)
	}
	return nil
}
func (p *SeasonAddSelection) UnmarshalJSON(data []byte) error {
	type plain SeasonAddSelection
	if err := requireMonitored(data); err != nil {
		return err
	}
	if err := rejectNull(data, "seasonNumber", "episodes"); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if _, ok := fields["seasonNumber"]; !ok {
		return fmt.Errorf("%w: seasonNumber is required", ErrValidation)
	}
	return json.Unmarshal(data, (*plain)(p))
}
func (p *EpisodeAddSelection) UnmarshalJSON(data []byte) error {
	type plain EpisodeAddSelection
	if err := requireMonitored(data); err != nil {
		return err
	}
	if err := rejectNull(data, "episodeNumber"); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if _, ok := fields["episodeNumber"]; !ok {
		return fmt.Errorf("%w: episodeNumber is required", ErrValidation)
	}
	return json.Unmarshal(data, (*plain)(p))
}
