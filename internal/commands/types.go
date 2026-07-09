package commands

type arrItem struct {
	ID               int    `json:"id"`
	Title            string `json:"title"`
	Year             int    `json:"year"`
	Status           string `json:"status"`
	Network          string `json:"network"`
	Monitored        bool   `json:"monitored"`
	QualityProfileID int    `json:"qualityProfileId"`
	Overview         string `json:"overview"`
	TVDBID           int    `json:"tvdbId"`
	TMDBID           int    `json:"tmdbId"`
	Seasons          []any  `json:"seasons"`
	Images           []any  `json:"images"`
	Tags             []int  `json:"tags"`
	RootFolderPath   string `json:"rootFolderPath"`
	SeriesID         int    `json:"seriesId"`
	EpisodeNumber    int    `json:"episodeNumber"`
	SeasonNumber     int    `json:"seasonNumber"`
	AirDateUTC       string `json:"airDateUtc"`
	DigitalRelease   string `json:"digitalRelease"`
	InCinemas        string `json:"inCinemas"`
	PhysicalRelease  string `json:"physicalRelease"`
	MovieFileID      int    `json:"movieFileId"`
	MovieFile        *movieFile `json:"movieFile"`
}

type movieFile struct {
	ID           int    `json:"id"`
	RelativePath string `json:"relativePath"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	Quality      struct {
		Quality struct {
			Name string `json:"name"`
		} `json:"quality"`
	} `json:"quality"`
}

type episodeFile struct {
	ID           int    `json:"id"`
	RelativePath string `json:"relativePath"`
	Path         string `json:"path"`
}

type tag struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

type profile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type rootFolder struct {
	Path string `json:"path"`
}

type calendarRow struct {
	Date    string `json:"date"`
	Title   string `json:"title"`
	Episode string `json:"episode"`
	Service string `json:"service"`
}
