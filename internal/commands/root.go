package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jfriend615/arrctl/internal/api"
	"github.com/jfriend615/arrctl/internal/config"
	"github.com/jfriend615/arrctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	cfgPath string
	format  string
	quiet   bool
	version = "0.2.0"
)

func Execute() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{Use: "arrctl", Short: "Unified CLI for *arr", SilenceUsage: true}
	root.PersistentFlags().StringVar(&cfgPath, "config", "", "Config file")
	root.PersistentFlags().StringVar(&format, "format", "auto", "json|table|auto")
	root.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode")
	root.Version = version
	root.AddCommand(sonarrCmd(), radarrCmd(), overseerrCmd(), tautulliCmd(), calendarCmd(), completionCmd(root))
	return root
}

func completionCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{Use: "completion [bash|zsh|fish|powershell]", Short: "Generate shell completion", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return root.GenBashCompletion(os.Stdout)
		case "zsh":
			return root.GenZshCompletion(os.Stdout)
		case "fish":
			return root.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return root.GenPowerShellCompletion(os.Stdout)
		default:
			return fmt.Errorf("unsupported shell")
		}
	}}
	return cmd
}

type arrItem struct {
	ID               int
	Title            string
	Year             int
	Status           string
	Monitored        bool
	QualityProfileID int
	Overview         string
	TVDBID           int
	TMDBID           int
	Network          string
	Seasons          []any
	Images           []any
	SeriesID         int
	EpisodeNumber    int
	SeasonNumber     int
	AirDateUTC       string
	DigitalRelease   string
	InCinemas        string
	PhysicalRelease  string
	MovieFileID      int
}
type profile struct {
	ID   int
	Name string
}
type rootFolder struct{ Path string }

func serviceClient(name string) (*api.Client, config.Service, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, config.Service{}, err
	}
	s, err := cfg.MustService(name)
	if err != nil {
		return nil, config.Service{}, err
	}
	return api.New(s.URL, s.APIKey), s, nil
}

func render(v any, headers []string, rows [][]string) error {
	m := output.Mode(format)
	if m == output.JSON || !output.IsTable(m) {
		return output.PrintJSON(v)
	}
	output.PrintTable(headers, rows)
	return nil
}

func sonarrCmd() *cobra.Command { return arrCmd("sonarr", "series", "tvdb", true) }
func radarrCmd() *cobra.Command { return arrCmd("radarr", "movie", "tmdb", false) }

func arrCmd(service, noun, idType string, hasEpisodes bool) *cobra.Command {
	cmd := &cobra.Command{Use: service}
	cmd.AddCommand(
		arrListCmd(service, noun),
		arrSearchCmd(service, noun),
		arrAddCmd(service, noun, idType, hasEpisodes),
		arrInfoCmd(service, noun),
		arrDeleteCmd(service, noun),
		arrCalendarCmd(service, hasEpisodes),
	)
	return cmd
}

func arrListCmd(service, noun string) *cobra.Command {
	var monitored, unmonitored bool
	cmd := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := serviceClient(service)
		if err != nil {
			return err
		}
		var items []arrItem
		if err := c.Do(context.Background(), "GET", "/api/v3/"+noun, nil, &items); err != nil {
			return err
		}
		filtered := []arrItem{}
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
	cmd := &cobra.Command{Use: "search <term>", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := serviceClient(service)
		if err != nil {
			return err
		}
		term := strings.Join(args, " ")
		var items []arrItem
		if err := c.Do(context.Background(), "GET", fmt.Sprintf("/api/v3/%s/lookup?term=%s", noun, term), nil, &items); err != nil {
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
			rows = append(rows, output.ToStrings(id, it.Title, it.Year, it.Status))
		}
		h := []string{"TVDB ID", "Title", "Year", "Status"}
		if noun == "movie" {
			h[0] = "TMDB ID"
		}
		return render(items, h, rows)
	}}
	cmd.Flags().IntVar(&limit, "limit", 10, "")
	return cmd
}

func arrAddCmd(service, noun, idType string, showSeason bool) *cobra.Command {
	var id, quality, root string
	var search, monitored bool
	cmd := &cobra.Command{Use: "add", RunE: func(cmd *cobra.Command, args []string) error {
		if id == "" {
			return errors.New("--id required")
		}
		c, s, err := serviceClient(service)
		if err != nil {
			return err
		}
		var look []arrItem
		if err := c.Do(context.Background(), "GET", fmt.Sprintf("/api/v3/%s/lookup?term=%s:%s", noun, idType, id), nil, &look); err != nil {
			return err
		}
		if len(look) == 0 {
			return errors.New("not found")
		}
		item := look[0]
		qid, err := resolveQuality(c, quality, s.Defaults.QualityProfile)
		if err != nil {
			return err
		}
		rp, err := resolveRoot(c, root, s.Defaults.RootFolder)
		if err != nil {
			return err
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
		if err := c.Do(context.Background(), "POST", "/api/v3/"+noun, payload, &created); err != nil {
			return err
		}
		if format == "json" {
			return output.PrintJSON(created)
		}
		if !quiet {
			fmt.Fprintf(os.Stderr, "Added %s\n", item.Title)
		}
		return nil
	}}
	cmd.Flags().StringVar(&id, "id", "", "")
	cmd.Flags().StringVar(&quality, "quality", "", "")
	cmd.Flags().StringVar(&root, "root", "", "")
	cmd.Flags().BoolVar(&search, "search", false, "")
	cmd.Flags().BoolVar(&monitored, "monitored", true, "")
	cmd.Flags().Lookup("monitored").NoOptDefVal = "true"
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
	cmd := &cobra.Command{Use: "info", RunE: func(cmd *cobra.Command, args []string) error {
		if id == 0 && name == "" {
			return errors.New("--id or --name required")
		}
		c, _, err := serviceClient(service)
		if err != nil {
			return err
		}
		var items []arrItem
		if err := c.Do(context.Background(), "GET", "/api/v3/"+noun, nil, &items); err != nil {
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
		rows := [][]string{}
		for _, it := range matched {
			rows = append(rows, output.ToStrings(it.ID, it.Title, it.Year, it.Status, map[bool]string{true: "Yes", false: "No"}[it.Monitored], it.Overview))
		}
		return render(matched, []string{"ID", "Title", "Year", "Status", "Monitored", "Overview"}, rows)
	}}
	cmd.Flags().IntVar(&id, "id", 0, "")
	cmd.Flags().StringVar(&name, "name", "", "")
	return cmd
}

func arrDeleteCmd(service, noun string) *cobra.Command {
	var id int
	var deleteFiles, addExclusion, yes bool
	cmd := &cobra.Command{Use: "delete", RunE: func(cmd *cobra.Command, args []string) error {
		if id == 0 {
			return errors.New("--id required")
		}
		if !yes {
			return errors.New("confirmation required; pass --yes")
		}
		c, _, err := serviceClient(service)
		if err != nil {
			return err
		}
		ep := fmt.Sprintf("/api/v3/%s/%d?deleteFiles=%t&addImportListExclusion=%t", noun, id, deleteFiles, addExclusion)
		if err := c.Do(context.Background(), "DELETE", ep, nil, nil); err != nil {
			return err
		}
		if format == "json" {
			return output.PrintJSON(map[string]any{"service": service, "deleted": true, "id": id})
		}
		if !quiet {
			fmt.Fprintf(os.Stderr, "Deleted %s %d\n", service, id)
		}
		return nil
	}}
	cmd.Flags().IntVar(&id, "id", 0, "")
	cmd.Flags().BoolVar(&deleteFiles, "delete-files", false, "")
	cmd.Flags().BoolVar(&addExclusion, "add-exclusion", false, "")
	cmd.Flags().BoolVar(&yes, "yes", false, "")
	return cmd
}

func arrCalendarCmd(service string, sonarr bool) *cobra.Command {
	var days int
	var start, end string
	cmd := &cobra.Command{Use: "calendar", RunE: func(cmd *cobra.Command, args []string) error {
		if start == "" {
			start = time.Now().Format("2006-01-02")
		}
		if end == "" {
			end = time.Now().AddDate(0, 0, days).Format("2006-01-02")
		}
		c, _, err := serviceClient(service)
		if err != nil {
			return err
		}
		var items []arrItem
		if err := c.Do(context.Background(), "GET", fmt.Sprintf("/api/v3/calendar?start=%s&end=%s", start, end), nil, &items); err != nil {
			return err
		}
		type row struct{ Date, Title, Episode, Service string }
		rowsObj := []row{}
		if sonarr {
			var series []arrItem
			_ = c.Do(context.Background(), "GET", "/api/v3/series", nil, &series)
			lookup := map[int]string{}
			for _, s := range series {
				lookup[s.ID] = s.Title
			}
			for _, e := range items {
				t := lookup[e.SeriesID]
				if t == "" {
					t = "Unknown Series"
				}
				rowsObj = append(rowsObj, row{Date: strings.Split(e.AirDateUTC, "T")[0], Title: t, Episode: fmt.Sprintf("S%02dE%02d", e.SeasonNumber, e.EpisodeNumber), Service: "Sonarr"})
			}
		} else {
			for _, m := range items {
				d := firstDate(m.DigitalRelease, m.InCinemas, m.PhysicalRelease)
				if d == "" {
					continue
				}
				rowsObj = append(rowsObj, row{Date: strings.Split(d, "T")[0], Title: m.Title, Episode: "", Service: "Radarr"})
			}
		}
		rows := [][]string{}
		for _, r := range rowsObj {
			rows = append(rows, []string{r.Date, r.Title, r.Episode, r.Service})
		}
		return render(rowsObj, []string{"Date", "Title", "Episode", "Service"}, rows)
	}}
	cmd.Flags().IntVar(&days, "days", 7, "")
	cmd.Flags().StringVar(&start, "start", "", "")
	cmd.Flags().StringVar(&end, "end", "", "")
	return cmd
}

func calendarCmd() *cobra.Command {
	var days int
	var week, onlySonarr, onlyRadarr bool
	cmd := &cobra.Command{Use: "calendar", RunE: func(cmd *cobra.Command, args []string) error {
		start := time.Now()
		end := start.AddDate(0, 0, days)
		if week {
			wd := int(start.Weekday())
			if wd == 0 {
				wd = 7
			}
			start = start.AddDate(0, 0, -(wd - 1))
			end = start.AddDate(0, 0, 6)
		}
		svcs := []string{"sonarr", "radarr"}
		if onlySonarr {
			svcs = []string{"sonarr"}
		}
		if onlyRadarr {
			svcs = []string{"radarr"}
		}
		type row struct{ Date, Title, Episode, Service string }
		all := []row{}
		for _, s := range svcs {
			c := arrCalendarCmd(s, s == "sonarr")
			_ = c.Flags().Set("start", start.Format("2006-01-02"))
			_ = c.Flags().Set("end", end.Format("2006-01-02"))
			buf := &capture{val: &all, svc: s}
			_ = buf.run()
		}
		sort.Slice(all, func(i, j int) bool { return all[i].Date < all[j].Date })
		if len(all) == 0 && !quiet {
			fmt.Fprintf(os.Stderr, "No releases in this period\n")
		}
		rows := [][]string{}
		for _, r := range all {
			rows = append(rows, []string{r.Date, r.Title, r.Episode, r.Service})
		}
		return render(all, []string{"Date", "Title", "Episode", "Service"}, rows)
	}}
	cmd.Flags().IntVar(&days, "days", 7, "")
	cmd.Flags().BoolVar(&week, "week", false, "")
	cmd.Flags().BoolVar(&onlySonarr, "sonarr", false, "")
	cmd.Flags().BoolVar(&onlyRadarr, "radarr", false, "")
	return cmd
}

type capture struct {
	val any
	svc string
}

func (c *capture) run() error {
	client, _, err := serviceClient(c.svc)
	if err != nil {
		return nil
	}
	start := time.Now().Format("2006-01-02")
	end := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	var items []arrItem
	if err := client.Do(context.Background(), "GET", fmt.Sprintf("/api/v3/calendar?start=%s&end=%s", start, end), nil, &items); err != nil {
		return nil
	}
	if c.svc == "sonarr" {
		var series []arrItem
		_ = client.Do(context.Background(), "GET", "/api/v3/series", nil, &series)
		lookup := map[int]string{}
		for _, s := range series {
			lookup[s.ID] = s.Title
		}
		for _, e := range items {
			*c.val.(*[]struct{ Date, Title, Episode, Service string }) = append(*c.val.(*[]struct{ Date, Title, Episode, Service string }), struct{ Date, Title, Episode, Service string }{Date: strings.Split(e.AirDateUTC, "T")[0], Title: lookup[e.SeriesID], Episode: fmt.Sprintf("S%02dE%02d", e.SeasonNumber, e.EpisodeNumber), Service: "Sonarr"})
		}
	} else {
		for _, m := range items {
			if d := firstDate(m.DigitalRelease, m.InCinemas, m.PhysicalRelease); d != "" {
				*c.val.(*[]struct{ Date, Title, Episode, Service string }) = append(*c.val.(*[]struct{ Date, Title, Episode, Service string }), struct{ Date, Title, Episode, Service string }{Date: strings.Split(d, "T")[0], Title: m.Title, Service: "Radarr"})
			}
		}
	}
	return nil
}

func firstDate(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func resolveQuality(c *api.Client, explicit, def string) (int, error) {
	var p []profile
	if err := c.Do(context.Background(), "GET", "/api/v3/qualityprofile", nil, &p); err != nil {
		return 0, err
	}
	pick := explicit
	if pick == "" {
		pick = def
	}
	if pick != "" {
		if n, err := strconv.Atoi(pick); err == nil {
			for _, pr := range p {
				if pr.ID == n {
					return n, nil
				}
			}
		}
		for _, pr := range p {
			if pr.Name == pick {
				return pr.ID, nil
			}
		}
		return 0, fmt.Errorf("quality profile not found: %s", pick)
	}
	if len(p) == 0 {
		return 0, errors.New("no quality profiles available")
	}
	return p[0].ID, nil
}
func resolveRoot(c *api.Client, explicit, def string) (string, error) {
	var r []rootFolder
	if err := c.Do(context.Background(), "GET", "/api/v3/rootfolder", nil, &r); err != nil {
		return "", err
	}
	pick := explicit
	if pick == "" {
		pick = def
	}
	if pick != "" {
		for _, rr := range r {
			if rr.Path == pick {
				return pick, nil
			}
		}
		return "", fmt.Errorf("root folder not found: %s", pick)
	}
	if len(r) == 0 {
		return "", errors.New("no root folders configured")
	}
	return r[0].Path, nil
}

func overseerrCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "overseerr"}
	cmd.AddCommand(&cobra.Command{Use: "pending", RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := serviceClient("overseerr")
		if err != nil {
			return err
		}
		var out map[string]any
		if err := c.Do(context.Background(), "GET", "/api/v1/request?filter=pending&take=100", nil, &out); err != nil {
			return err
		}
		return output.PrintJSON(out["results"])
	}})
	approve := &cobra.Command{Use: "approve <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := serviceClient("overseerr")
		if err != nil {
			return err
		}
		msg, _ := cmd.Flags().GetString("message")
		body := map[string]string{}
		if msg != "" {
			body["message"] = msg
		}
		return c.Do(context.Background(), "POST", "/api/v1/request/"+args[0]+"/approve", body, nil)
	}}
	approve.Flags().String("message", "", "")
	deny := &cobra.Command{Use: "deny <id>", Aliases: []string{"decline"}, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := serviceClient("overseerr")
		if err != nil {
			return err
		}
		r, _ := cmd.Flags().GetString("reason")
		body := map[string]string{}
		if r != "" {
			body["reason"] = r
		}
		return c.Do(context.Background(), "POST", "/api/v1/request/"+args[0]+"/decline", body, nil)
	}}
	deny.Flags().String("reason", "", "")
	cmd.AddCommand(approve, deny)
	return cmd
}

func tautulliCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "tautulli"}
	cmd.AddCommand(&cobra.Command{Use: "now", RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := serviceClient("tautulli")
		if err != nil {
			return err
		}
		var resp struct {
			Response struct {
				Result, Message string
				Data            struct {
					Sessions []map[string]any `json:"sessions"`
				} `json:"data"`
			} `json:"response"`
		}
		if err := c.Tautulli(context.Background(), "get_activity", nil, &resp); err != nil {
			return err
		}
		if resp.Response.Result != "success" {
			return fmt.Errorf("tautulli api error: %s", resp.Response.Message)
		}
		if len(resp.Response.Data.Sessions) == 0 {
			if !quiet {
				fmt.Fprintln(os.Stderr, "No active streams")
			}
			os.Exit(1)
		}
		if quiet {
			return nil
		}
		return output.PrintJSON(resp.Response.Data.Sessions)
	}})
	stale := &cobra.Command{Use: "stale", RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := serviceClient("tautulli")
		if err != nil {
			return err
		}
		lib, _ := cmd.Flags().GetString("library")
		minDays, _ := cmd.Flags().GetInt("min-days")
		maxPlays, _ := cmd.Flags().GetInt("max-plays")
		minSize, _ := cmd.Flags().GetFloat64("min-size-gb")
		limit, _ := cmd.Flags().GetInt("limit")
		var lresp struct {
			Response struct {
				Result, Message string
				Data            []map[string]any `json:"data"`
			} `json:"response"`
		}
		if err := c.Tautulli(context.Background(), "get_libraries", nil, &lresp); err != nil {
			os.Exit(2)
		}
		if lresp.Response.Result != "success" {
			os.Exit(2)
		}
		selected := []map[string]any{}
		for _, li := range lresp.Response.Data {
			if lib == "" || strings.EqualFold(fmt.Sprint(li["section_name"]), lib) || fmt.Sprint(li["section_id"]) == lib {
				selected = append(selected, li)
			}
		}
		if len(selected) == 0 {
			os.Exit(2)
		}
		items := []map[string]any{}
		for _, li := range selected {
			var mresp struct {
				Response struct {
					Result string
					Data   struct {
						Data []map[string]any `json:"data"`
					} `json:"data"`
				} `json:"response"`
			}
			_ = c.Tautulli(context.Background(), "get_library_media_info", map[string]string{"section_id": fmt.Sprint(li["section_id"]), "length": "10000"}, &mresp)
			items = append(items, mresp.Response.Data.Data...)
		}
		now := time.Now().Unix()
		out := []map[string]any{}
		for _, it := range items {
			size := toInt64(it["file_size"])
			plays := int(toInt64(it["play_count"]))
			last := toInt64(it["last_played"])
			days := 999999
			if last > 0 {
				days = int((now - last) / 86400)
			}
			if float64(size) < minSize*1073741824 || plays > maxPlays || days < minDays {
				continue
			}
			it["days_since_last_played"] = days
			it["size_gb"] = float64(size) / 1073741824
			out = append(out, it)
		}
		if len(out) > limit {
			out = out[:limit]
		}
		if len(out) == 0 {
			if !quiet {
				fmt.Fprintln(os.Stderr, "No stale candidates found")
			}
			os.Exit(1)
		}
		return output.PrintJSON(out)
	}}
	stale.Flags().String("library", "", "")
	stale.Flags().Int("min-days", 180, "")
	stale.Flags().Int("max-plays", 2, "")
	stale.Flags().Float64("min-size-gb", 1, "")
	stale.Flags().Int("limit", 50, "")
	cmd.AddCommand(stale)
	return cmd
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	default:
		return 0
	}
}
