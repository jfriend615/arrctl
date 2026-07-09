package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/jfriend615/arrctl/internal/api"
	"github.com/jfriend615/arrctl/internal/output"
	"github.com/spf13/cobra"
)

func overseerrCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "overseerr",
		Short: "Manage media requests via Overseerr",
		Long: `arrctl overseerr - Manage media requests via Overseerr

Commands:
  pending    List pending requests awaiting approval
  approve    Approve a pending request
  deny       Deny or decline a request`,
		Example: `  arrctl overseerr pending
  arrctl overseerr approve 123
  arrctl overseerr deny 125 --reason "Already available"`,
	}
	cmd.AddCommand(&cobra.Command{Use: "pending", Short: "List pending requests", RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := serviceClient("overseerr")
		if err != nil {
			return err
		}
		var out map[string]any
		if err := c.Do(cmd.Context(), "GET", "/api/v1/request?filter=pending&take=100", nil, &out); err != nil {
			return err
		}
		results, ok := out["results"].([]any)
		if !ok {
			return output.PrintJSON(out["results"])
		}
		if len(results) == 0 {
			if !quiet {
				fmt.Fprintln(cmd.ErrOrStderr(), "No pending requests")
			}
			return nil
		}
		rows := [][]string{}
		for _, r := range results {
			req, ok := r.(map[string]any)
			if !ok {
				continue
			}
			media := toMap(req["media"])
			user := toMap(req["requestedBy"])
			title, err := resolveOverseerrRequestTitle(cmd.Context(), c, req)
			if err != nil {
				return err
			}
			date := fmt.Sprint(req["createdAt"])
			if idx := strings.Index(date, "T"); idx >= 0 {
				date = date[:idx]
			}
			requestUser := firstNonEmpty(user["displayName"], user["username"], "Unknown")
			rows = append(rows, output.ToStrings(
				req["id"],
				requestUser,
				title,
				media["mediaType"],
				date,
			))
		}
		return render(results, []string{"ID", "User", "Title", "Type", "Date"}, rows)
	}})
	approve := &cobra.Command{Use: "approve <id>", Short: "Approve a pending request", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := serviceClient("overseerr")
		if err != nil {
			return err
		}
		msg, _ := cmd.Flags().GetString("message")
		body := map[string]string{}
		if msg != "" {
			body["message"] = msg
		}
		if err := c.Do(cmd.Context(), "POST", "/api/v1/request/"+args[0]+"/approve", body, nil); err != nil {
			return err
		}
		if !quiet {
			fmt.Fprintf(cmd.ErrOrStderr(), "Approved request %s\n", args[0])
		}
		return nil
	}}
	approve.Flags().String("message", "", "")
	deny := &cobra.Command{Use: "deny <id>", Short: "Deny or decline a request", Aliases: []string{"decline"}, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := serviceClient("overseerr")
		if err != nil {
			return err
		}
		r, _ := cmd.Flags().GetString("reason")
		body := map[string]string{}
		if r != "" {
			body["reason"] = r
		}
		if err := c.Do(cmd.Context(), "POST", "/api/v1/request/"+args[0]+"/decline", body, nil); err != nil {
			return err
		}
		if !quiet {
			fmt.Fprintf(cmd.ErrOrStderr(), "Declined request %s\n", args[0])
		}
		return nil
	}}
	deny.Flags().String("reason", "", "")
	cmd.AddCommand(approve, deny)
	return cmd
}

func resolveOverseerrRequestTitle(ctx context.Context, c *api.Client, req map[string]any) (string, error) {
	media := toMap(req["media"])
	if title := firstNonEmpty(media["title"], media["name"], req["title"]); title != "" {
		return title, nil
	}
	mediaType := firstNonEmpty(req["type"], media["mediaType"])
	tmdbID := firstNonEmpty(media["tmdbId"])
	if mediaType == "" || tmdbID == "" {
		return "Unknown", nil
	}
	var detail map[string]any
	if err := c.Do(ctx, "GET", fmt.Sprintf("/api/v1/%s/%s", mediaType, tmdbID), nil, &detail); err != nil {
		return "", err
	}
	return firstNonEmpty(detail["title"], detail["name"], "Unknown"), nil
}
