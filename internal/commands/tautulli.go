package commands

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jfriend615/arrctl/internal/output"
	"github.com/spf13/cobra"
)

func tautulliCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tautulli",
		Short: "View Plex activity via Tautulli",
		Long: `arrctl tautulli - View Plex activity via Tautulli

Commands:
  now      Show who is currently streaming
  stale    Show stale media candidates (rarely watched + old)`,
		Example: `  arrctl tautulli now
  arrctl tautulli stale
  arrctl tautulli stale --library Movies --min-days 365 --max-plays 1 --min-size-gb 4`,
	}
	cmd.AddCommand(&cobra.Command{Use: "now", Short: "Show who is currently streaming", RunE: func(cmd *cobra.Command, args []string) error {
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
		if err := c.Tautulli(cmd.Context(), "get_activity", nil, &resp); err != nil {
			return exitErr(2, err)
		}
		if resp.Response.Result != "success" {
			return exitErr(2, fmt.Errorf("tautulli api error: %s", resp.Response.Message))
		}
		if len(resp.Response.Data.Sessions) == 0 {
			if !quiet {
				fmt.Fprintln(cmd.ErrOrStderr(), "No active streams")
			}
			return exitErr(1, nil)
		}
		if quiet {
			return nil
		}
		rows := [][]string{}
		for _, s := range resp.Response.Data.Sessions {
			rows = append(rows, output.ToStrings(
				firstNonEmpty(s["user"], s["friendly_name"], "Unknown"),
				firstNonEmpty(s["full_title"], s["title"], "Unknown"),
				fmt.Sprintf("%s%%", firstNonEmpty(s["progress_percent"], 0)),
				firstNonEmpty(s["video_full_resolution"], s["quality_profile"], "Unknown"),
				firstNonEmpty(s["transcode_decision"], "Unknown"),
				firstNonEmpty(s["state"], "Unknown"),
			))
		}
		return render(resp.Response.Data.Sessions, []string{"User", "Title", "Progress", "Quality", "Transcode", "State"}, rows)
	}})
	stale := &cobra.Command{Use: "stale", Short: "Show stale media candidates", RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := serviceClient("tautulli")
		if err != nil {
			return exitErr(2, err)
		}
		lib, _ := cmd.Flags().GetString("library")
		minDays, _ := cmd.Flags().GetInt("min-days")
		maxPlays, _ := cmd.Flags().GetInt("max-plays")
		minSize, _ := cmd.Flags().GetFloat64("min-size-gb")
		limit, _ := cmd.Flags().GetInt("limit")
		jsonFlag, _ := cmd.Flags().GetBool("json")
		useJSON := format == "json" || jsonFlag
		if limit <= 0 {
			return exitErr(2, errors.New("--limit must be > 0"))
		}
		var lresp struct {
			Response struct {
				Result, Message string
				Data            []map[string]any `json:"data"`
			} `json:"response"`
		}
		if err := c.Tautulli(cmd.Context(), "get_libraries", nil, &lresp); err != nil {
			return exitErr(2, err)
		}
		if lresp.Response.Result != "success" {
			return exitErr(2, errors.New(lresp.Response.Message))
		}
		selected := []map[string]any{}
		for _, li := range lresp.Response.Data {
			if lib == "" || strings.EqualFold(fmt.Sprint(li["section_name"]), lib) || fmt.Sprint(li["section_id"]) == lib {
				selected = append(selected, li)
			}
		}
		if len(selected) == 0 {
			return exitErr(2, fmt.Errorf("library not found: %s", lib))
		}
		items := []map[string]any{}
		for _, li := range selected {
			libraryName := firstNonEmpty(li["section_name"], "Unknown")
			var mresp struct {
				Response struct {
					Result, Message string
					Data            struct {
						Data []map[string]any `json:"data"`
					} `json:"data"`
				} `json:"response"`
			}
			if err := c.Tautulli(cmd.Context(), "get_library_media_info", map[string]string{"section_id": fmt.Sprint(li["section_id"]), "length": "10000"}, &mresp); err != nil {
				return exitErr(2, err)
			}
			if mresp.Response.Result != "success" {
				return exitErr(2, errors.New(mresp.Response.Message))
			}
			for _, it := range mresp.Response.Data.Data {
				if isNilish(it["library_name"]) {
					it["library_name"] = libraryName
				}
				if isNilish(it["section_name"]) {
					it["section_name"] = firstNonEmpty(it["library_name"], libraryName)
				}
				items = append(items, it)
			}
		}
		now := time.Now().Unix()
		out := []map[string]any{}
		for _, it := range items {
			size := toInt64(it["file_size"])
			plays := int(toInt64(it["play_count"]))
			last := toInt64(it["last_played"])
			added := toInt64(it["added_at"])
			days := daysSinceLastPlayed(now, last, added)
			if float64(size) < minSize*1073741824 || plays > maxPlays || days < minDays {
				continue
			}
			sizeGB := math.Floor((float64(size)/1073741824)*100) / 100
			staleScore := computeStaleScore(sizeGB, days, plays, maxPlays, last, added, now)
			it["file_size"] = size
			it["play_count"] = plays
			it["last_played"] = last
			it["added_at"] = added
			it["days_since_last_played"] = days
			it["size_gb"] = sizeGB
			it["stale_score"] = staleScore
			if isNilish(it["section_name"]) {
				it["section_name"] = firstNonEmpty(it["library_name"], "Unknown")
			}
			if isNilish(it["title"]) {
				it["title"] = firstNonEmpty(it["sort_title"], "Unknown")
			}
			out = append(out, it)
		}
		sort.Slice(out, func(i, j int) bool {
			si, sj := toFloat64(out[i]["stale_score"]), toFloat64(out[j]["stale_score"])
			if si != sj {
				return si > sj
			}
			fi, fj := toInt64(out[i]["file_size"]), toInt64(out[j]["file_size"])
			if fi != fj {
				return fi > fj
			}
			return toInt64(out[i]["last_played"]) < toInt64(out[j]["last_played"])
		})
		if len(out) > limit {
			out = out[:limit]
		}
		if len(out) == 0 {
			if !quiet {
				fmt.Fprintln(cmd.ErrOrStderr(), "No stale candidates found")
			}
			return exitErr(1, nil)
		}
		if useJSON {
			return output.PrintJSON(out)
		}
		rows := [][]string{}
		totalSize := int64(0)
		for _, it := range out {
			totalSize += toInt64(it["file_size"])
			lastPlayed := "Never"
			if toInt64(it["last_played"]) > 0 {
				lastPlayed = time.Unix(toInt64(it["last_played"]), 0).Format("2006-01-02")
			}
			dateAdded := "Unknown"
			if toInt64(it["added_at"]) > 0 {
				dateAdded = time.Unix(toInt64(it["added_at"]), 0).Format("2006-01-02")
			}
			rows = append(rows, output.ToStrings(
				firstNonEmpty(it["section_name"], it["library_name"], "Unknown"),
				firstNonEmpty(it["media_type"], "unknown"),
				firstNonEmpty(it["title"], it["sort_title"], "Unknown"),
				fmt.Sprintf("%.2f", it["size_gb"]),
				it["play_count"],
				lastPlayed,
				dateAdded,
			))
		}
		if err := render(out, []string{"Library", "Type", "Title", "Size(GB)", "Plays", "Last Played", "Date Added"}, rows); err != nil {
			return err
		}
		if output.IsTable(output.Mode(format)) && !quiet {
			fmt.Fprintf(cmd.ErrOrStderr(), "%d stale candidate(s) | total size: %.2f GiB\n", len(out), float64(totalSize)/1073741824)
		}
		return nil
	}}
	stale.Flags().String("library", "", "")
	stale.Flags().Int("min-days", 180, "")
	stale.Flags().Int("max-plays", 2, "")
	stale.Flags().Float64("min-size-gb", 1, "")
	stale.Flags().Int("limit", 50, "")
	stale.Flags().Bool("json", false, "")
	cmd.AddCommand(stale)
	return cmd
}

func firstNonEmpty(vals ...any) string {
	for _, v := range vals {
		if s := normalizeString(v); s != "" {
			return s
		}
	}
	return ""
}

func isNilish(v any) bool {
	return normalizeString(v) == ""
}

func normalizeString(v any) string {
	if v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return ""
	}
	return s
}

func computeStaleScore(sizeGB float64, daysSinceLastPlayed, plays, maxPlays int, lastPlayed, addedAt, nowEpoch int64) float64 {
	const (
		sizeGBCap  = 20.0
		ageDaysCap = 3650.0 // 10 years
	)

	sizeNorm := clamp01(sizeGB / sizeGBCap)

	ageDays := float64(daysSinceLastPlayed)
	if lastPlayed <= 0 {
		switch {
		case addedAt > 0:
			ageDays = math.Max(0, float64((nowEpoch-addedAt)/86400))
		default:
			ageDays = ageDaysCap
		}
	}
	ageNorm := clamp01(ageDays / ageDaysCap)

	denominator := float64(maxPlays + 1)
	if denominator <= 0 {
		denominator = 1
	}
	playNorm := clamp01(1 - (float64(plays) / denominator))

	neverPlayedBonus := 0.0
	if lastPlayed <= 0 {
		neverPlayedBonus = 0.15
	}

	return (sizeNorm * 0.4) + (ageNorm * 0.4) + (playNorm * 0.2) + neverPlayedBonus
}

func daysSinceLastPlayed(nowEpoch, lastPlayed, addedAt int64) int {
	if lastPlayed > 0 {
		return int(math.Max(0, float64((nowEpoch-lastPlayed)/86400)))
	}
	if addedAt > 0 {
		return int(math.Max(0, float64((nowEpoch-addedAt)/86400)))
	}
	return 999999
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func toFloat64(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		f := 0.0
		fmt.Sscanf(t, "%f", &f)
		return f
	default:
		return float64(toInt64(v))
	}
}
