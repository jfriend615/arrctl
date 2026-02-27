package commands

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/jfriend615/arrctl/internal/output"
	"github.com/spf13/cobra"
)

func sonarrCmd() *cobra.Command { return arrCmd("sonarr", "series", "tvdb") }
func radarrCmd() *cobra.Command { return arrCmd("radarr", "movie", "tmdb") }

func arrCmd(service, noun, idType string) *cobra.Command {
	cmd := &cobra.Command{Use: service}
	cmd.AddCommand(arrListCmd(service, noun), arrSearchCmd(service, noun), arrAddCmd(service, noun, idType), arrInfoCmd(service, noun), arrDeleteCmd(service, noun), arrCalendarCmd(service, noun == "series"))
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
	cmd := &cobra.Command{Use: "search <term>", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := serviceClient(service)
		if err != nil {
			return err
		}
		term := url.QueryEscape(strings.Join(args, " "))
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

func arrAddCmd(service, noun, idType string) *cobra.Command {
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
		ctx := cmd.Context()
		var look []arrItem
		if err := c.Do(ctx, "GET", fmt.Sprintf("/api/v3/%s/lookup?term=%s:%s", noun, idType, id), nil, &look); err != nil {
			return err
		}
		if len(look) == 0 {
			return errors.New("not found")
		}
		item := look[0]
		qid, err := resolveQuality(ctx, c, quality, s.Defaults.QualityProfile)
		if err != nil {
			return err
		}
		rp, err := resolveRoot(ctx, c, root, s.Defaults.RootFolder)
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
		if err := c.Do(ctx, "POST", "/api/v3/"+noun, payload, &created); err != nil {
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
		if err := c.Do(cmd.Context(), "DELETE", ep, nil, nil); err != nil {
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
