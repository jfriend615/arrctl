package commands

import (
	"github.com/jfriend615/arrctl/internal/output"
	"github.com/spf13/cobra"
)

func overseerrCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "overseerr"}
	cmd.AddCommand(&cobra.Command{Use: "pending", RunE: func(cmd *cobra.Command, args []string) error {
		c, _, err := serviceClient("overseerr")
		if err != nil {
			return err
		}
		var out map[string]any
		if err := c.Do(cmd.Context(), "GET", "/api/v1/request?filter=pending&take=100", nil, &out); err != nil {
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
		return c.Do(cmd.Context(), "POST", "/api/v1/request/"+args[0]+"/approve", body, nil)
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
		return c.Do(cmd.Context(), "POST", "/api/v1/request/"+args[0]+"/decline", body, nil)
	}}
	deny.Flags().String("reason", "", "")
	cmd.AddCommand(approve, deny)
	return cmd
}
