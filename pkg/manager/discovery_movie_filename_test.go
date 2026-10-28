package manager

import "testing"

func TestMovieDiscoveryFilename(t *testing.T) {
	tests := []struct {
		folder, raw string
		siblings    []string
		want        string
		keep        bool
	}{
		{"Toy Story", "Toy Story 4.mkv", nil, "Toy Story 4", true},
		{"Hunger Games", "The Hunger Games Mockingjay Part 1.mkv", nil, "The Hunger Games Mockingjay Part 1", true},
		{"Other Movie", "Unrelated Part 1.mkv", nil, "Unrelated Part 1", true},
		{"Film", "Film interview.mkv", []string{"Film.mkv"}, "Film interview", false},
		{"Film", "Film deleted scenes.mkv", []string{"Film.mkv"}, "Film deleted scenes", false},
		{"Film", "Film trailer.mkv", []string{"Film.mkv"}, "Film trailer", false},
		{"The Interview", "The Interview.mkv", nil, "The Interview", true},
		{"Bonus", "Bonus.mkv", nil, "Bonus", true},
		{"Toy Story", "Toy Story 1.mkv", []string{"Toy Story.mkv", "Toy Story 1.mkv", "Toy Story 2.mkv"}, "Toy Story 1", true},
		{"Toy Story", "Toy Story 2.mkv", []string{"Toy Story.mkv", "Toy Story 1.mkv", "Toy Story 2.mkv"}, "Toy Story 2", true},
		{"Toy Story", "Toy Story-4.mkv", []string{"Toy Story.mkv"}, "Toy Story-4", true},
		{"Toy Story", "Toy Story-1.mkv", nil, "Toy Story-1", true},
		{"Toy Story", "Toy Story-1.mkv", []string{"Toy Story.mkv", "Toy Story-1.mkv", "Toy Story-2.mkv"}, "Toy Story", true},
		{"Toy Story", "Toy Story-2.mkv", []string{"Toy Story.mkv", "Toy Story-1.mkv", "Toy Story-2.mkv"}, "Toy Story", true},
		{"WALL-E", "Wall E-1.mp4", []string{"Wall E-1.mp4", "Wall E-2.mp4", "Wall E.mkv"}, "Wall E", true},
		{"WALL-E", "Wall E-2.mp4", []string{"Wall E-1.mp4", "Wall E-2.mp4", "Wall E.mkv"}, "Wall E", true},
		{"WALL-E", "Wall E.mkv", []string{"Wall E-1.mp4", "Wall E-2.mp4", "Wall E.mkv"}, "Wall E", true},
		{"Rosencrantz and Guildenstern are Dead (1990)", "Gary Oldman interview.avi", []string{"Rosencrantz and Guildenstern are Dead (1990).mkv"}, "Gary Oldman interview", false},
		{"Alien", "Interview with the Vampire (1994).mkv", nil, "Interview with the Vampire (1994)", true},
		{"Alien", "The Interview (2014).mkv", nil, "The Interview (2014)", true},
		{"Rosencrantz and Guildenstern are Dead (1990)", "Behind-the-scenes.mp4", []string{"Rosencrantz and Guildenstern are Dead (1990).mkv"}, "Behind-the-scenes", false},
		{"Rosencrantz and Guildenstern are Dead (1990)", "Deleted.Scenes.mp4", []string{"Rosencrantz and Guildenstern are Dead (1990).mkv"}, "Deleted.Scenes", false},
		{"The Interview (2014)", "The Interview (2014).mkv", nil, "The Interview (2014)", true},
		{"Alien {tmdb-123}", "Alien {tmdb-456}.mkv", []string{"Alien {tmdb-456}.mkv"}, "Alien {tmdb-456}", true},
		{"Interview with the Vampire", "Interview with the Vampire (1994).mkv", nil, "Interview with the Vampire (1994)", true},
		{"Film", "Film 2020.mkv", nil, "Film 2020", true},
		{"Movie", "Toy Story 4.mkv", nil, "Toy Story 4", true},
	}
	for _, tc := range tests {
		got, keep := movieDiscoveryFilename(tc.folder, tc.raw, tc.siblings)
		if got != tc.want || keep != tc.keep {
			t.Errorf("movieDiscoveryFilename(%q): got (%q,%v), want (%q,%v)", tc.raw, got, keep, tc.want, tc.keep)
		}
	}
}
