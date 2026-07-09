package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"strings"

	"github.com/jfriend615/arrctl/internal/api"
	"github.com/jfriend615/arrctl/internal/output"
	"github.com/spf13/cobra"
)

func sonarrCmd() *cobra.Command { return arrCmd("sonarr", "series", "tvdb") }
func radarrCmd() *cobra.Command { return arrCmd("radarr", "movie", "tmdb") }

func arrCmd(service, noun, idType string) *cobra.Command {
	short := map[string]string{"sonarr": "Manage Sonarr (TV shows)", "radarr": "Manage Radarr (Movies)"}[service]
	long := map[string]string{
		"sonarr": `arrctl sonarr - Manage Sonarr (TV shows)

Commands:
  list        List all series in the library
  search      Search for series by name
  add         Add a series to the library
  info        Show detailed series information
  delete      Delete a series from the library
  calendar    Show upcoming episodes`,
		"radarr": `arrctl radarr - Manage Radarr (Movies)

Commands:
  list        List all movies in the library
  search      Search for movies by name
  add         Add a movie to the library
  info        Show detailed movie information
  delete      Delete a movie from the library
  calendar    Show upcoming movie releases`,
	}[service]
	cmd := &cobra.Command{
		Use:   service,
		Short: short,
		Long:  long,
		Example: fmt.Sprintf(`  arrctl %s list
  arrctl %s search "Example"
  arrctl %s info --name "Example"`, service, service, service),
	}
	cmd.AddCommand(arrListCmd(service, noun), arrSearchCmd(service, noun), arrAddCmd(service, noun, idType), arrInfoCmd(service, noun), arrDeleteCmd(service, noun), arrCalendarCmd(service, noun == "series"))
	return cmd
}

func arrListCmd(service, noun string) *cobra.Command {
	var monitored, unmonitored bool
	cmd := &cobra.Command{Use: "list", Short: "List library items", RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := serviceClient(service)
		if err != nil {
			return err
		}
		if format == "json" {
			var raw []map[string]any
			if err := c.Do(cmd.Context(), "GET", "/api/v3/"+noun, nil, &raw); err != nil {
				return err
			}
			filtered := make([]map[string]any, 0, len(raw))
			for _, it := range raw {
				monitoredVal := false
				if v, ok := it["monitored"].(bool); ok {
					monitoredVal = v
				}
				if monitored && !monitoredVal {
					continue
				}
				if unmonitored && monitoredVal {
					continue
				}
				filtered = append(filtered, it)
			}
			return output.PrintJSON(filtered)
		}
		var items []arrItem
		if err := c.Do(cmd.Context(), "GET", "/api/v3/"+noun, nil, &items); err != nil {
			return err
		}
		filtered := make([]arrItem, 0, len(items))
		for _, it := range items {
			if monitored && !it.Monitored {
				continue
			}
			if unmonitored && it.Monitored {
				continue
			}
			filtered = append(filtered, it)
		}
		rows := [][]string{}
		for _, it := range filtered {
			rows = append(rows, output.ToStrings(it.ID, it.Title, it.Year, it.Status, map[bool]string{true: "Yes", false: "No"}[it.Monitored]))
		}
		return render(filtered, []string{"ID", "Title", "Year", "Status", "Monitored"}, rows)
	}}
	cmd.Flags().BoolVar(&monitored, "monitored", false, "")
	cmd.Flags().BoolVar(&unmonitored, "unmonitored", false, "")
	return cmd
}

func arrSearchCmd(service, noun string) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "search <term>", Short: "Search for media by name", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if limit < 0 {
			return errors.New("--limit must be >= 0")
		}
		c, _, err := serviceClient(service)
		if err != nil {
			return err
		}
		term := url.QueryEscape(strings.Join(args, " "))
		if format == "json" {
			var raw []map[string]any
			if err := c.Do(cmd.Context(), "GET", fmt.Sprintf("/api/v3/%s/lookup?term=%s", noun, term), nil, &raw); err != nil {
				return err
			}
			if len(raw) > limit {
				raw = raw[:limit]
			}
			return output.PrintJSON(raw)
		}
		var items []arrItem
		if err := c.Do(cmd.Context(), "GET", fmt.Sprintf("/api/v3/%s/lookup?term=%s", noun, term), nil, &items); err != nil {
			return err
		}
		if len(items) > limit {
			items = items[:limit]
		}
		rows := [][]string{}
		for _, it := range items {
			id := it.TVDBID
			if noun == "movie" {
				id = it.TMDBID
			}
			if noun == "series" {
				rows = append(rows, output.ToStrings(id, it.Title, it.Year, it.Network, it.Status))
				continue
			}
			rows = append(rows, output.ToStrings(id, it.Title, it.Year, it.Status))
		}
		h := []string{"TVDB ID", "Title", "Year", "Network", "Status"}
		if noun == "movie" {
			h = []string{"TMDB ID", "Title", "Year", "Status"}
		}
		return render(items, h, rows)
	}}
	cmd.Flags().IntVar(&limit, "limit", 10, "")
	return cmd
}

func arrAddCmd(service, noun, idType string) *cobra.Command {
	var id, quality, root string
	var search, monitored bool
	cmd := &cobra.Command{Use: "add", Short: "Add media to the library", RunE: func(cmd *cobra.Command, args []string) error {
		if id == "" {
			return errors.New("--id required")
		}
		c, s, err := serviceClient(service)
		if err != nil {
			return err
		}
		ctx := cmd.Context()
		if !quiet {
			if noun == "series" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Looking up series with TVDB ID: %s\n", id)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Looking up movie with TMDb ID: %s\n", id)
			}
		}
		var look []arrItem
		if err := c.Do(ctx, "GET", fmt.Sprintf("/api/v3/%s/lookup?term=%s:%s", noun, idType, id), nil, &look); err != nil {
			return err
		}
		if len(look) == 0 {
			if noun == "series" {
				return fmt.Errorf("no series found with TVDB ID: %s", id)
			}
			return fmt.Errorf("no movie found with TMDb ID: %s", id)
		}
		item := look[0]
		if !quiet {
			fmt.Fprintf(cmd.ErrOrStderr(), "Found: %s\n", item.Title)
		}
		qid, err := resolveQuality(ctx, c, quality, s.Defaults.QualityProfile)
		if err != nil {
			return err
		}
		if !quiet {
			fmt.Fprintf(cmd.ErrOrStderr(), "Using quality profile ID: %d\n", qid)
		}
		rp, err := resolveRoot(ctx, c, root, s.Defaults.RootFolder)
		if err != nil {
			return err
		}
		if !quiet {
			fmt.Fprintf(cmd.ErrOrStderr(), "Using root folder: %s\n", rp)
		}
		payload := map[string]any{"title": item.Title, "qualityProfileId": qid, "rootFolderPath": rp, "monitored": monitored, "images": item.Images}
		if noun == "series" {
			payload["tvdbId"] = item.TVDBID
			payload["seasonFolder"] = true
			payload["seasons"] = item.Seasons
			payload["addOptions"] = map[string]bool{"searchForMissingEpisodes": search}
		} else {
			payload["tmdbId"] = item.TMDBID
			payload["addOptions"] = map[string]bool{"searchForMovie": search}
		}
		var created map[string]any
		if err := c.Do(ctx, "POST", "/api/v3/"+noun, payload, &created); err != nil {
			return err
		}
		if !quiet {
			createdID := toInt64(created["id"])
			fmt.Fprintf(cmd.ErrOrStderr(), "Successfully added: %s (ID: %d)\n", item.Title, createdID)
		}
		if format == "json" {
			return output.PrintJSON(created)
		}
		return nil
	}}
	cmd.Flags().StringVar(&id, "id", "", "")
	cmd.Flags().StringVar(&quality, "quality", "", "")
	cmd.Flags().StringVar(&root, "root", "", "")
	cmd.Flags().BoolVar(&search, "search", false, "")
	cmd.Flags().BoolVar(&monitored, "monitored", true, "")
	cmd.Flags().Bool("no-monitored", false, "")
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		if v, _ := cmd.Flags().GetBool("no-monitored"); v {
			monitored = false
		}
	}
	return cmd
}

func arrInfoCmd(service, noun string) *cobra.Command {
	var id int
	var name string
	cmd := &cobra.Command{Use: "info", Short: "Show detailed information", RunE: func(cmd *cobra.Command, args []string) error {
		if id != 0 && name != "" {
			return errors.New("use either --id or --name, not both")
		}
		if id == 0 && name == "" {
			return errors.New("either --id or --name is required")
		}
		c, _, err := serviceClient(service)
		if err != nil {
			return err
		}
		var items []arrItem
		if err := c.Do(cmd.Context(), "GET", "/api/v3/"+noun, nil, &items); err != nil {
			return err
		}
		matched := []arrItem{}
		for _, it := range items {
			if id != 0 && it.ID == id {
				matched = append(matched, it)
			}
			if name != "" && strings.Contains(strings.ToLower(it.Title), strings.ToLower(name)) {
				matched = append(matched, it)
			}
		}
		if len(matched) == 0 {
			return fmt.Errorf("no matching %s found", noun)
		}
		if noun == "series" {
			return renderSeriesInfo(cmd, c, matched)
		}
		return renderMovieInfo(cmd, c, matched)
	}}
	cmd.Flags().IntVar(&id, "id", 0, "")
	cmd.Flags().StringVar(&name, "name", "", "")
	return cmd
}

func renderSeriesInfo(cmd *cobra.Command, c *api.Client, matched []arrItem) error {
	ctx := cmd.Context()
	profiles, tags, err := loadProfileAndTagLookups(ctx, c)
	if err != nil {
		return err
	}
	out := make([]map[string]any, 0, len(matched))
	rows := make([][]string, 0, len(matched))
	for _, it := range matched {
		var episodes []map[string]any
		if err := c.Do(ctx, "GET", fmt.Sprintf("/api/v3/episode?seriesId=%d", it.ID), nil, &episodes); err != nil {
			return err
		}
		var episodeFiles []episodeFile
		if err := c.Do(ctx, "GET", fmt.Sprintf("/api/v3/episodefile?seriesId=%d", it.ID), nil, &episodeFiles); err != nil {
			return err
		}
		files := make([]string, 0, len(episodeFiles))
		for _, ef := range episodeFiles {
			switch {
			case ef.RelativePath != "":
				files = append(files, ef.RelativePath)
			case ef.Path != "":
				files = append(files, ef.Path)
			default:
				files = append(files, fmt.Sprintf("ID:%d", ef.ID))
			}
		}
		entry := map[string]any{
			"id":                 it.ID,
			"title":              it.Title,
			"year":               it.Year,
			"status":             it.Status,
			"monitored":          it.Monitored,
			"qualityProfileName": profiles[it.QualityProfileID],
			"rootFolder":         it.RootFolderPath,
			"overview":           it.Overview,
			"tags":               tagLabels(it.Tags, tags),
			"seasonsCount":       len(it.Seasons),
			"episodesCount":      len(episodes),
			"episodeFiles":       files,
		}
		if entry["qualityProfileName"] == "" {
			entry["qualityProfileName"] = "Unknown"
		}
		out = append(out, entry)
		rows = append(rows, output.ToStrings(
			it.ID,
			it.Title,
			it.Year,
			it.Status,
			map[bool]string{true: "Yes", false: "No"}[it.Monitored],
			entry["qualityProfileName"],
			it.RootFolderPath,
			len(it.Seasons),
			len(episodes),
			strings.Join(entry["tags"].([]string), ", "),
			strings.Join(files, ", "),
			it.Overview,
		))
	}
	return render(out, []string{"ID", "Title", "Year", "Status", "Monitored", "Quality Profile", "Root Folder", "Seasons", "Episodes", "Tags", "Episode Files", "Overview"}, rows)
}

func renderMovieInfo(cmd *cobra.Command, c *api.Client, matched []arrItem) error {
	ctx := cmd.Context()
	profiles, tags, err := loadProfileAndTagLookups(ctx, c)
	if err != nil {
		return err
	}
	out := make([]map[string]any, 0, len(matched))
	rows := make([][]string, 0, len(matched))
	for _, it := range matched {
		var mf *movieFile
		movieFileID := 0
		switch {
		case it.MovieFile != nil && it.MovieFile.ID != 0:
			mf = it.MovieFile
			movieFileID = mf.ID
		case it.MovieFileID != 0:
			movieFileID = it.MovieFileID
		}
		if movieFileID != 0 && mf == nil {
			var fetched movieFile
			if err := c.Do(ctx, "GET", fmt.Sprintf("/api/v3/moviefile/%d", movieFileID), nil, &fetched); err != nil {
				return err
			}
			mf = &fetched
		}
		entry := map[string]any{
			"id":                 it.ID,
			"title":              it.Title,
			"year":               it.Year,
			"status":             it.Status,
			"monitored":          it.Monitored,
			"qualityProfileName": profiles[it.QualityProfileID],
			"rootFolder":         it.RootFolderPath,
			"overview":           it.Overview,
			"tags":               tagLabels(it.Tags, tags),
			"movieFile":          mf,
		}
		if entry["qualityProfileName"] == "" {
			entry["qualityProfileName"] = "Unknown"
		}
		out = append(out, entry)
		rows = append(rows, output.ToStrings(
			it.ID,
			it.Title,
			it.Year,
			it.Status,
			map[bool]string{true: "Yes", false: "No"}[it.Monitored],
			entry["qualityProfileName"],
			it.RootFolderPath,
			strings.Join(entry["tags"].([]string), ", "),
			formatMovieFileSummary(mf),
			it.Overview,
		))
	}
	return render(out, []string{"ID", "Title", "Year", "Status", "Monitored", "Quality Profile", "Root Folder", "Tags", "Movie File", "Overview"}, rows)
}

func loadProfileAndTagLookups(ctx context.Context, c *api.Client) (map[int]string, map[int]string, error) {
	var profiles []profile
	if err := c.Do(ctx, "GET", "/api/v3/qualityprofile", nil, &profiles); err != nil {
		return nil, nil, err
	}
	var tags []tag
	if err := c.Do(ctx, "GET", "/api/v3/tag", nil, &tags); err != nil {
		return nil, nil, err
	}
	profileLookup := make(map[int]string, len(profiles))
	for _, p := range profiles {
		profileLookup[p.ID] = p.Name
	}
	tagLookup := make(map[int]string, len(tags))
	for _, t := range tags {
		tagLookup[t.ID] = t.Label
	}
	return profileLookup, tagLookup, nil
}

func tagLabels(ids []int, lookup map[int]string) []string {
	if len(ids) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if label := lookup[id]; label != "" {
			out = append(out, label)
		} else {
			out = append(out, fmt.Sprintf("Tag-%d", id))
		}
	}
	return out
}

func formatMovieFileSummary(mf *movieFile) string {
	if mf == nil {
		return "Not Downloaded"
	}
	path := mf.RelativePath
	if path == "" {
		path = mf.Path
	}
	sizeGB := math.Floor(float64(mf.Size) / 1073741824)
	quality := mf.Quality.Quality.Name
	if quality == "" {
		quality = "Unknown"
	}
	return fmt.Sprintf("%s | %.0f GB | %s", path, sizeGB, quality)
}

func arrDeleteCmd(service, noun string) *cobra.Command {
	var id int
	var deleteFiles, addExclusion, yes bool
	cmd := &cobra.Command{Use: "delete", Short: "Delete media from the library", RunE: func(cmd *cobra.Command, args []string) error {
		if id == 0 {
			return errors.New("--id required")
		}
		c, _, err := serviceClient(service)
		if err != nil {
			return err
		}
		var current arrItem
		if err := c.Do(cmd.Context(), "GET", fmt.Sprintf("/api/v3/%s/%d", noun, id), nil, &current); err != nil {
			return err
		}
		if !yes {
			confirmed, err := confirmDeletion(service, noun, current.Title, id)
			if err != nil {
				return err
			}
			if !confirmed {
				if !quiet {
					fmt.Fprintln(cmd.ErrOrStderr(), "Cancelled")
				}
				return nil
			}
		}
		ep := fmt.Sprintf("/api/v3/%s/%d?deleteFiles=%t&addImportListExclusion=%t", noun, id, deleteFiles, addExclusion)
		if err := c.Do(cmd.Context(), "DELETE", ep, nil, nil); err != nil {
			return err
		}
		if format == "json" {
			return output.PrintJSON(map[string]any{
				"service":                service,
				"deleted":                true,
				"id":                     id,
				"title":                  current.Title,
				"deleteFiles":            deleteFiles,
				"addImportListExclusion": addExclusion,
			})
		}
		if !quiet {
			label := map[string]string{"series": "series", "movie": "movie"}[noun]
			if label == "" {
				label = noun
			}
			fmt.Fprintf(os.Stderr, "Deleted %s %s: %s (ID: %d)\n", strings.Title(service), label, current.Title, id)
		}
		return nil
	}}
	cmd.Flags().IntVar(&id, "id", 0, "")
	cmd.Flags().BoolVar(&deleteFiles, "delete-files", false, "")
	cmd.Flags().BoolVar(&addExclusion, "add-exclusion", false, "")
	cmd.Flags().BoolVar(&yes, "yes", false, "")
	return cmd
}

func confirmDeletion(service, noun, title string, id int) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
		tty, err := os.Open("/dev/tty")
		if err == nil {
			defer tty.Close()
			reader = bufio.NewReader(tty)
		}
	}
	fmt.Fprintf(os.Stderr, "Delete %s %s %q (ID: %d)? [y/N]: ", strings.Title(service), noun, title, id)
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
