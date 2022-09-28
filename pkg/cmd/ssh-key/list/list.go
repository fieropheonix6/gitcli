package list

import (
	"net/http"
	"time"

	"github.com/cli/cli/v2/internal/config"
	"github.com/cli/cli/v2/internal/text"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/cli/cli/v2/utils"
	"github.com/spf13/cobra"
)

type ListOptions struct {
	IO         *iostreams.IOStreams
	Config     func() (config.Config, error)
	HTTPClient func() (*http.Client, error)
}

func NewCmdList(f *cmdutil.Factory, runF func(*ListOptions) error) *cobra.Command {
	opts := &ListOptions{
		IO:         f.IOStreams,
		Config:     f.Config,
		HTTPClient: f.HttpClient,
	}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "Lists SSH keys in your GitHub account",
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}
			return listRun(opts)
		},
	}

	return cmd
}

func listRun(opts *ListOptions) error {
	apiClient, err := opts.HTTPClient()
	if err != nil {
		return err
	}

	cfg, err := opts.Config()
	if err != nil {
		return err
	}

	host, _ := cfg.DefaultHost()

	sshKeys, err := userKeys(apiClient, host, "")
	if err != nil {
		return err
	}

	if len(sshKeys) == 0 {
		return cmdutil.NewNoResultsError("no SSH keys present in the GitHub account")
	}

	t := utils.NewTablePrinter(opts.IO)
	cs := opts.IO.ColorScheme()
	now := time.Now()

	for _, sshKey := range sshKeys {
		t.AddField(sshKey.Title, nil, nil)
		t.AddField(sshKey.Key, truncateMiddle, nil)

		createdAt := sshKey.CreatedAt.Format(time.RFC3339)
		if t.IsTTY() {
			createdAt = text.FuzzyAgoAbbr(now, sshKey.CreatedAt)
		}
		t.AddField(createdAt, nil, cs.Gray)
		t.EndRow()
	}

	return t.Render()
}

func truncateMiddle(maxWidth int, t string) string {
	if len(t) <= maxWidth {
		return t
	}

	ellipsis := "..."
	if maxWidth < len(ellipsis)+2 {
		return t[0:maxWidth]
	}

	halfWidth := (maxWidth - len(ellipsis)) / 2
	remainder := (maxWidth - len(ellipsis)) % 2
	return t[0:halfWidth+remainder] + ellipsis + t[len(t)-halfWidth:]
}
