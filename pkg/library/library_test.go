package library

import (
	"context"
	"slices"
	"testing"
	"testing/fstest"
)

func TestMatchMovie(t *testing.T) {
	_, expected := MovieFSFromFile(t, "./test_movies.txt")

	for _, m := range expected {
		matched := matchMovie(m)
		if !matched {
			t.Errorf("didn't match movie: %s", m)
		}
	}
}

func TestMatchEpisode(t *testing.T) {
	_, expected := TVFSFromFile(t, "./test_episodes.txt")

	for _, m := range expected {
		matched := matchEpisode(m)
		if !matched {
			t.Errorf("didn't match episode: %s", m)
		}
	}
}

func TestFindMovies(t *testing.T) {
	ctx := context.Background()
	negatives := fstest.MapFS{
		"myfile.txt": {},
		"Batman Begins (2005)/Batman Begins (2005).en.srt": {},
		"My Movie/Uh Oh/My Movie.mp4":                      {},
	}
	fs, expected := MovieFSFromFile(t, "./test_movies.txt")
	for k, v := range negatives {
		fs[k] = v
	}

	l := New(fs, nil)
	movies, err := l.FindMovies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	movieNames := make([]string, len(movies))
	for i, m := range movies {
		movieNames[i] = m.Name
	}
	if !slices.Equal(expected, movieNames) {
		t.Fatalf("wanted %v; got %v", expected, movieNames)
	}

}

func TestFindEpisodes(t *testing.T) {
	ctx := context.Background()
	fs, expected := TVFSFromFile(t, "./test_episodes.txt")
	fs["myfile.txt"] = &fstest.MapFile{}

	l := New(nil, fs)
	episodes, err := l.FindEpisodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(expected, episodes) {
		t.Fatalf("wanted %v; got %v", expected, episodes)
	}
}

func Test_fileSizeToString(t *testing.T) {
	type args struct {
		size int64
	}
	tests := []struct {
		name string
		want string
		args args
	}{
		{
			name: "b",
			args: args{size: 500},
			want: "500 B",
		},
		{
			name: "kb",
			args: args{size: 1024},
			want: "1.0 KB",
		},
		{
			name: "mb",
			args: args{size: 1048576}, // 1024 * 1024
			want: "1.0 MB",
		},
		{
			name: "gb",
			args: args{size: 1073741824}, // 1024 * 1024 * 1024
			want: "1.0 GB",
		},
		{
			name: "tb",
			args: args{size: 1099511627776}, // 1024^4
			want: "1.0 TB",
		},
		{
			name: "not whole number",
			args: args{size: 123456789},
			want: "117.7 MB",
		},
		{
			name: "zero",
			args: args{size: 0},
			want: "0 B",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fileSizeToString(tt.args.size); got != tt.want {
				t.Errorf("fileSizeToString() = %v, want %v", got, tt.want)
			}
		})
	}
}
