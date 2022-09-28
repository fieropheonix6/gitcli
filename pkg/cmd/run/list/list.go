package list

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/internal/text"
	"github.com/cli/cli/v2/pkg/cmd/run/shared"
	workflowShared "github.com/cli/cli/v2/pkg/cmd/workflow/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/cli/cli/v2/utils"
	"github.com/spf13/cobra"
)

const (
	defaultLimit = 20
)

type ListOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (ghrepo.Interface, error)

	Exporter cmdutil.Exporter

	Limit            int
	WorkflowSelector string
	Branch           string
	Actor            string

	now time.Time
}

func NewCmdList(f *cmdutil.Factory, runF func(*ListOptions) error) *cobra.Command {
	opts := &ListOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		now:        time.Now(),
	}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List recent workflow runs",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// support `-R, --repo` override
			opts.BaseRepo = f.BaseRepo

			if opts.Limit < 1 {
				return cmdutil.FlagErrorf("invalid limit: %v", opts.Limit)
			}

			if runF != nil {
				return runF(opts)
			}

			return listRun(opts)
		},
	}

	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", defaultLimit, "Maximum number of runs to fetch")
	cmd.Flags().StringVarP(&opts.WorkflowSelector, "workflow", "w", "", "Filter runs by workflow")
	cmd.Flags().StringVarP(&opts.Branch, "branch", "b", "", "Filter runs by branch")
	cmd.Flags().StringVarP(&opts.Actor, "user", "u", "", "Filter runs by user who triggered the run")
	cmdutil.AddJSONFlags(cmd, &opts.Exporter, shared.RunFields)

	return cmd
}

func listRun(opts *ListOptions) error {
	baseRepo, err := opts.BaseRepo()
	if err != nil {
		return fmt.Errorf("failed to determine base repo: %w", err)
	}

	c, err := opts.HttpClient()
	if err != nil {
		return fmt.Errorf("failed to create http client: %w", err)
	}
	client := api.NewClientFromHTTP(c)

	var runs []shared.Run
	var workflow *workflowShared.Workflow

	filters := &shared.FilterOptions{
		Branch: opts.Branch,
		Actor:  opts.Actor,
	}

	opts.IO.StartProgressIndicator()
	if opts.WorkflowSelector != "" {
		states := []workflowShared.WorkflowState{workflowShared.Active}
		workflow, err = workflowShared.ResolveWorkflow(
			opts.IO, client, baseRepo, false, opts.WorkflowSelector, states)
		if err == nil {
			runs, err = shared.GetRunsByWorkflow(client, baseRepo, filters, opts.Limit, workflow.ID)
		}
	} else {
		runs, err = shared.GetRuns(client, baseRepo, filters, opts.Limit)
	}
	opts.IO.StopProgressIndicator()
	if err != nil {
		return fmt.Errorf("failed to get runs: %w", err)
	}

	if err := opts.IO.StartPager(); err == nil {
		defer opts.IO.StopPager()
	} else {
		fmt.Fprintf(opts.IO.ErrOut, "failed to start pager: %v\n", err)
	}

	if opts.Exporter != nil {
		return opts.Exporter.Write(opts.IO, runs)
	}

	if len(runs) == 0 {
		return cmdutil.NewNoResultsError("no runs found")
	}

	tp := utils.NewTablePrinter(opts.IO)

	cs := opts.IO.ColorScheme()

	if tp.IsTTY() {
		tp.AddField("STATUS", nil, nil)
		tp.AddField("NAME", nil, nil)
		tp.AddField("WORKFLOW", nil, nil)
		tp.AddField("BRANCH", nil, nil)
		tp.AddField("EVENT", nil, nil)
		tp.AddField("ID", nil, nil)
		tp.AddField("ELAPSED", nil, nil)
		tp.AddField("AGE", nil, nil)
		tp.EndRow()
	}

	for _, run := range runs {
		if tp.IsTTY() {
			symbol, symbolColor := shared.Symbol(cs, run.Status, run.Conclusion)
			tp.AddField(symbol, nil, symbolColor)
		} else {
			tp.AddField(string(run.Status), nil, nil)
			tp.AddField(string(run.Conclusion), nil, nil)
		}

		tp.AddField(run.CommitMsg(), nil, cs.Bold)

		tp.AddField(run.Name, nil, nil)
		tp.AddField(run.HeadBranch, nil, cs.Bold)
		tp.AddField(string(run.Event), nil, nil)
		tp.AddField(fmt.Sprintf("%d", run.ID), nil, cs.Cyan)

		tp.AddField(run.Duration(opts.now).String(), nil, nil)
		tp.AddField(text.FuzzyAgoAbbr(time.Now(), run.StartedTime()), nil, nil)
		tp.EndRow()
	}

	err = tp.Render()
	if err != nil {
		return err
	}

	return nil
}
