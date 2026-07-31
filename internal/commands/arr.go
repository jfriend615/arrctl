package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"strconv"
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
		Args:  parentCommandArgs(service),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		Example: fmt.Sprintf(`  arrctl %s list
  arrctl %s search "Example"
  arrctl %s info --name "Example"`, service, service, service),
	}
	cmd.AddCommand(arrListCmd(service, noun), arrSearchCmd(service, noun), arrAddCmd(service, noun, idType), arrInfoCmd(service, noun), arrDeleteCmd(service, noun), arrCalendarCmd(service, noun == "series"))
	return cmd
}

func arrListCmd(service, noun string) *cobra.Command {
	var monitored, unmonitored bool
	cmd := &cobra.Command{Use: "list", Short: "List library items", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if monitored && unmonitored {
			return errors.New("--monitored and --unmonitored cannot be used together")
		}
		c, _, err := serviceClient(service)
		if err != nil {
			return err
		}
		var raw []map[string]any
		if err := c.Do(cmd.Context(), "GET", "/api/v3/"+noun, nil, &raw); err != nil {
			return err
		}
		filtered := make([]map[string]any, 0, len(raw))
		for _, it := range raw {
			monitoredValue, _ := it["monitored"].(bool)
			if monitored && !monitoredValue {
				continue
			}
			if unmonitored && monitoredValue {
				continue
			}
			filtered = append(filtered, it)
		}
		rows := [][]string{}
		for _, it := range filtered {
			monitoredValue, _ := it["monitored"].(bool)
			rows = append(rows, output.ToStrings(it["id"], it["title"], it["year"], it["status"], map[bool]string{true: "Yes", false: "No"}[monitoredValue]))
		}
		return render(filtered, []string{"ID", "Title", "Year", "Status", "Monitored"}, rows)
	}}
	cmd.Flags().BoolVar(&monitored, "monitored", false, "Show only monitored items")
	cmd.Flags().BoolVar(&unmonitored, "unmonitored", false, "Show only unmonitored items")
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
		var items []map[string]any
		if err := c.Do(cmd.Context(), "GET", fmt.Sprintf("/api/v3/%s/lookup?term=%s", noun, term), nil, &items); err != nil {
			return err
		}
		if len(items) > limit {
			items = items[:limit]
		}
		rows := [][]string{}
		for _, it := range items {
			id := it["tvdbId"]
			if noun == "movie" {
				id = it["tmdbId"]
			}
			if noun == "series" {
				rows = append(rows, output.ToStrings(id, it["title"], it["year"], firstNonEmpty(it["network"], "N/A"), it["status"]))
				continue
			}
			rows = append(rows, output.ToStrings(id, it["title"], it["year"], it["status"]))
		}
		h := []string{"TVDB ID", "Title", "Year", "Network", "Status"}
		if noun == "movie" {
			h = []string{"TMDb ID", "Title", "Year", "Status"}
		}
		return render(items, h, rows)
	}}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of results")
	return cmd
}

func arrAddCmd(service, noun, idType string) *cobra.Command {
	var id, quality, root string
	var search, monitored bool
	cmd := &cobra.Command{Use: "add", Short: "Add media to the library", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
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
		qid, err := resolveQuality(ctx, c, quality, s.Defaults.QualityProfile, cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		if !quiet {
			fmt.Fprintf(cmd.ErrOrStderr(), "Using quality profile ID: %d\n", qid)
		}
		rp, err := resolveRoot(ctx, c, root, s.Defaults.RootFolder, cmd.ErrOrStderr())
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
	cmd.Flags().StringVar(&id, "id", "", strings.ToUpper(idType)+" ID to add")
	cmd.Flags().StringVar(&quality, "quality", "", "Quality profile name or ID")
	cmd.Flags().StringVar(&root, "root", "", "Root folder path")
	cmd.Flags().BoolVar(&search, "search", false, "Start a search after adding")
	cmd.Flags().BoolVar(&monitored, "monitored", true, "Monitor the added item")
	cmd.Flags().Bool("no-monitored", false, "Do not monitor the added item")
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		if v, _ := cmd.Flags().GetBool("no-monitored"); v {
			monitored = false
		}
	}
	return cmd
}

func arrInfoCmd(service, noun string) *cobra.Command {
	var id string
	var name string
	cmd := &cobra.Command{Use: "info", Short: "Show detailed information", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if id != "" && name != "" {
			return errors.New("Use either --id or --name, not both")
		}
		if id == "" && name == "" {
			return errors.New("Either --id or --name is required")
		}
		parsedID := 0
		if id != "" {
			var err error
			parsedID, err = parseMediaID(id, service, noun)
			if err != nil {
				return err
			}
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
			if id != "" && it.ID == parsedID {
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
	cmd.Flags().StringVar(&id, "id", "", "Exact library item ID")
	cmd.Flags().StringVar(&name, "name", "", "Partial title match")
	return cmd
}

func renderSeriesInfo(cmd *cobra.Command, c *api.Client, matched []arrItem) error {
	ctx := cmd.Context()
	profiles, err := loadProfileLookup(ctx, c)
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
		entry := map[string]any{
			"id":                 it.ID,
			"title":              it.Title,
			"year":               it.Year,
			"status":             it.Status,
			"monitored":          it.Monitored,
			"qualityProfileName": profiles[it.QualityProfileID],
			"overview":           it.Overview,
			"seasonsCount":       len(it.Seasons),
			"episodesCount":      len(episodes),
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
			len(it.Seasons),
			len(episodes),
			it.Overview,
		))
	}
	return render(out, []string{"ID", "Title", "Year", "Status", "Monitored", "Quality Profile", "Seasons", "Episodes", "Overview"}, rows)
}

func renderMovieInfo(cmd *cobra.Command, c *api.Client, matched []arrItem) error {
	ctx := cmd.Context()
	profiles, err := loadProfileLookup(ctx, c)
	if err != nil {
		return err
	}
	out := make([]map[string]any, 0, len(matched))
	rows := make([][]string, 0, len(matched))
	for _, it := range matched {
		mf := it.MovieFile
		movieFileID := it.MovieFileID
		if mf != nil && mf.ID != 0 {
			movieFileID = mf.ID
		}
		if movieFileID != 0 {
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
			"overview":           it.Overview,
			"movieSizeGb":        movieSizeGB(mf),
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
			formatMovieSize(entry["movieSizeGb"]),
			it.Overview,
		))
	}
	return render(out, []string{"ID", "Title", "Year", "Status", "Monitored", "Quality Profile", "Size (GB)", "Overview"}, rows)
}

func loadProfileLookup(ctx context.Context, c *api.Client) (map[int]string, error) {
	var profiles []profile
	if err := c.Do(ctx, "GET", "/api/v3/qualityprofile", nil, &profiles); err != nil {
		return nil, err
	}
	profileLookup := make(map[int]string, len(profiles))
	for _, p := range profiles {
		profileLookup[p.ID] = p.Name
	}
	return profileLookup, nil
}

func movieSizeGB(mf *movieFile) any {
	if mf == nil {
		return nil
	}
	return math.Round(float64(mf.Size)/1073741824*10) / 10
}

func formatMovieSize(v any) string {
	if v == nil {
		return "Not Downloaded"
	}
	return fmt.Sprintf("%v GB", v)
}

func arrDeleteCmd(service, noun string) *cobra.Command {
	var id string
	var deleteFiles, addExclusion, yes bool
	cmd := &cobra.Command{Use: "delete", Short: "Delete media from the library", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if id == "" {
			return errors.New("--id is required")
		}
		parsedID, err := parseMediaID(id, service, noun)
		if err != nil {
			return err
		}
		c, _, err := serviceClient(service)
		if err != nil {
			return err
		}
		var current arrItem
		if err := c.Do(cmd.Context(), "GET", fmt.Sprintf("/api/v3/%s/%d", noun, parsedID), nil, &current); err != nil {
			return err
		}
		if !yes {
			confirmed, err := confirmDeletion(service, noun, current.Title, parsedID, cmd.ErrOrStderr())
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
		ep := fmt.Sprintf("/api/v3/%s/%d?deleteFiles=%t&addImportListExclusion=%t", noun, parsedID, deleteFiles, addExclusion)
		if err := c.Do(cmd.Context(), "DELETE", ep, nil, nil); err != nil {
			return err
		}
		if format == "json" {
			return printJSON(map[string]any{
				"service":                service,
				"deleted":                true,
				"id":                     parsedID,
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
			fmt.Fprintf(cmd.ErrOrStderr(), "Deleted %s %s: %s (ID: %d)\n", strings.Title(service), label, current.Title, parsedID)
		}
		return nil
	}}
	cmd.Flags().StringVar(&id, "id", "", "Library item ID to delete")
	cmd.Flags().BoolVar(&deleteFiles, "delete-files", false, "Also delete media files from disk")
	cmd.Flags().BoolVar(&addExclusion, "add-exclusion", false, "Add an import-list exclusion")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	return cmd
}

func confirmDeletion(service, noun, title string, id int, prompt io.Writer) (bool, error) {
	return confirmDeletionWithTTY(service, noun, title, id, func() (io.ReadCloser, error) {
		return os.Open("/dev/tty")
	}, prompt)
}

func confirmDeletionWithTTY(service, noun, title string, id int, openTTY func() (io.ReadCloser, error), prompt io.Writer) (bool, error) {
	tty, err := openTTY()
	if err != nil {
		return false, errors.New("cannot prompt without a terminal; use --yes")
	}
	defer tty.Close()
	reader := bufio.NewReader(tty)
	fmt.Fprintf(prompt, "Delete %s %s %q (ID: %d)? [y/N]: ", strings.Title(service), noun, title, id)
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

func parseMediaID(raw, service, noun string) (int, error) {
	label := map[string]string{
		"sonarr": "Sonarr series",
		"radarr": "Radarr movie",
	}[service]
	if label == "" {
		label = strings.Title(service) + " " + noun
	}
	if raw == "" || strings.Trim(raw, "0123456789") != "" {
		return 0, fmt.Errorf("--id must be a numeric %s ID", label)
	}
	id, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("--id must be a numeric %s ID", label)
	}
	return id, nil
}
