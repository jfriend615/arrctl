package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func arrCalendarCmd(service string, sonarr bool) *cobra.Command {
	var days int
	var start, end string
	cmd := &cobra.Command{Use: "calendar", Short: "Show service-specific upcoming releases", RunE: func(cmd *cobra.Command, args []string) error {
		if start == "" {
			start = time.Now().Format("2006-01-02")
		}
		if end == "" {
			end = time.Now().AddDate(0, 0, days).Format("2006-01-02")
		}
		rows, err := fetchServiceCalendar(cmd.Context(), service, sonarr, start, end)
		if err != nil {
			return err
		}
		strRows := make([][]string, 0, len(rows))
		for _, r := range rows {
			strRows = append(strRows, []string{r.Date, r.Title, r.Episode, r.Service})
		}
		return render(rows, []string{"Date", "Title", "Episode", "Service"}, strRows)
	}}
	cmd.Flags().IntVar(&days, "days", 7, "")
	cmd.Flags().StringVar(&start, "start", "", "")
	cmd.Flags().StringVar(&end, "end", "", "")
	return cmd
}

func calendarCmd() *cobra.Command {
	var days int
	var week, onlySonarr, onlyRadarr bool
	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Show upcoming releases from Sonarr and Radarr",
		Long: `arrctl calendar - Unified calendar view for upcoming releases

Options:
  --days N    Show the next N days (default: 7)
  --week      Show this week (Monday to Sunday)
  --sonarr    Show only TV episodes
  --radarr    Show only movies`,
		Example: `  arrctl calendar
  arrctl calendar --week
  arrctl calendar --days 14 --sonarr
  arrctl calendar --radarr`,
		RunE: func(cmd *cobra.Command, args []string) error {
		start, end := calendarRange(time.Now(), days, week)
		svcs := []string{"sonarr", "radarr"}
		if onlySonarr {
			svcs = []string{"sonarr"}
		}
		if onlyRadarr {
			svcs = []string{"radarr"}
		}

		all := []calendarRow{}
		for _, s := range svcs {
			rows, err := fetchServiceCalendar(cmd.Context(), s, s == "sonarr", start, end)
			if err != nil {
				return err
			}
			all = append(all, rows...)
		}
		sort.Slice(all, func(i, j int) bool { return all[i].Date < all[j].Date })
		if len(all) == 0 && !quiet {
			fmt.Fprintf(cmd.ErrOrStderr(), "No releases in this period (%s to %s)\n", start, end)
		}
		strRows := make([][]string, 0, len(all))
		for _, r := range all {
			strRows = append(strRows, []string{r.Date, r.Title, r.Episode, r.Service})
		}
		return render(all, []string{"Date", "Title", "Episode", "Service"}, strRows)
	}}
	cmd.Flags().IntVar(&days, "days", 7, "")
	cmd.Flags().BoolVar(&week, "week", false, "")
	cmd.Flags().BoolVar(&onlySonarr, "sonarr", false, "")
	cmd.Flags().BoolVar(&onlyRadarr, "radarr", false, "")
	return cmd
}

func fetchServiceCalendar(ctx context.Context, service string, sonarr bool, start, end string) ([]calendarRow, error) {
	c, _, err := serviceClient(service)
	if err != nil {
		return nil, err
	}
	var items []arrItem
	if err := c.Do(ctx, "GET", fmt.Sprintf("/api/v3/calendar?start=%s&end=%s", start, end), nil, &items); err != nil {
		return nil, err
	}
	rows := []calendarRow{}
	if sonarr {
		var series []arrItem
		if err := c.Do(ctx, "GET", "/api/v3/series", nil, &series); err != nil {
			return nil, err
		}
		lookup := map[int]string{}
		for _, s := range series {
			lookup[s.ID] = s.Title
		}
		for _, e := range items {
			t := lookup[e.SeriesID]
			if t == "" {
				t = "Unknown Series"
			}
			ep := fmt.Sprintf("S%02dE%02d", e.SeasonNumber, e.EpisodeNumber)
			if e.Title != "" {
				ep += " - " + e.Title
			}
			rows = append(rows, calendarRow{Date: strings.Split(e.AirDateUTC, "T")[0], Title: t, Episode: ep, Service: "Sonarr"})
		}
	} else {
		for _, m := range items {
			d := firstDate(m.DigitalRelease, m.InCinemas, m.PhysicalRelease)
			if d == "" {
				continue
			}
			rows = append(rows, calendarRow{Date: strings.Split(d, "T")[0], Title: m.Title, Episode: "", Service: "Radarr"})
		}
	}
	return rows, nil
}

func calendarRange(now time.Time, days int, week bool) (string, string) {
	start := now
	end := start.AddDate(0, 0, days)
	if week {
		wd := int(start.Weekday())
		if wd == 0 {
			wd = 7
		}
		start = start.AddDate(0, 0, -(wd - 1))
		end = start.AddDate(0, 0, 6)
	}
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

func firstDate(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
