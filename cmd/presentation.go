package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/the20100/g-slides-cli/internal/output"
	slides "google.golang.org/api/slides/v1"
)

var presentationCmd = &cobra.Command{
	Use:   "presentation",
	Short: "Manage Google Slides presentations",
}

// ---- presentation create ----

var presentationCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new blank presentation",
	Long: `Create a new blank Google Slides presentation with the given title.

Examples:
  gslides presentation create "My Deck"
  gslides presentation create "Q4 Review" --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := svc.Presentations.Create(&slides.Presentation{
			Title: args[0],
		}).Do()
		if err != nil {
			return fmt.Errorf("creating presentation: %w", err)
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(p, output.IsPretty(cmd))
		}
		fmt.Printf("Presentation created: %s\n", p.Title)
		fmt.Printf("ID:     %s\n", p.PresentationId)
		fmt.Printf("Slides: %d\n", len(p.Slides))
		fmt.Printf("URL:    https://docs.google.com/presentation/d/%s/edit\n", p.PresentationId)
		return nil
	},
}

// ---- presentation get ----

var presentationGetCmd = &cobra.Command{
	Use:   "get <presentation-id>",
	Short: "Get details of a presentation",
	Long: `Get the full details of a Google Slides presentation.

Examples:
  gslides presentation get 1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgVE2upms
  gslides presentation get <id> --pretty`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := svc.Presentations.Get(args[0]).Do()
		if err != nil {
			return fmt.Errorf("getting presentation: %w", err)
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(p, output.IsPretty(cmd))
		}
		output.PrintKeyValue([][]string{
			{"ID", p.PresentationId},
			{"Title", p.Title},
			{"Locale", p.Locale},
			{"Slides", fmt.Sprintf("%d", len(p.Slides))},
			{"Masters", fmt.Sprintf("%d", len(p.Masters))},
			{"Layouts", fmt.Sprintf("%d", len(p.Layouts))},
			{"URL", "https://docs.google.com/presentation/d/" + p.PresentationId + "/edit"},
		})
		return nil
	},
}

// ---- presentation slides ----

var presentationSlidesCmd = &cobra.Command{
	Use:   "slides <presentation-id>",
	Short: "List all slides in a presentation",
	Long: `List all slides (pages) in a Google Slides presentation with their IDs and layout info.

Examples:
  gslides presentation slides <id>
  gslides presentation slides <id> --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := svc.Presentations.Get(args[0]).Do()
		if err != nil {
			return fmt.Errorf("getting presentation: %w", err)
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(p.Slides, output.IsPretty(cmd))
		}
		if len(p.Slides) == 0 {
			fmt.Println("No slides found.")
			return nil
		}
		headers := []string{"#", "SLIDE ID", "ELEMENTS", "LAYOUT"}
		rows := make([][]string, len(p.Slides))
		for i, s := range p.Slides {
			layout := "-"
			if s.SlideProperties != nil && s.SlideProperties.LayoutObjectId != "" {
				layout = output.Truncate(s.SlideProperties.LayoutObjectId, 24)
			}
			rows[i] = []string{
				fmt.Sprintf("%d", i+1),
				s.ObjectId,
				fmt.Sprintf("%d", len(s.PageElements)),
				layout,
			}
		}
		output.PrintTable(headers, rows)
		return nil
	},
}

// ---- presentation batch-update ----

var (
	batchUpdateFile  string
	batchUpdateStdin bool
)

var presentationBatchUpdateCmd = &cobra.Command{
	Use:   "batch-update <presentation-id>",
	Short: "Apply a batch of updates to a presentation",
	Long: `Apply one or more atomic updates to a presentation via the batchUpdate API.

The requests must be provided as a JSON array of Request objects, either from
a file (--file) or from stdin (--stdin).

Request types include: createSlide, createShape, createImage, createTable,
insertText, deleteObject, duplicateObject, replaceAllText, and many more.
See: https://developers.google.com/workspace/slides/api/reference/rest/v1/presentations/batchUpdate

Examples:
  # From a JSON file
  gslides presentation batch-update <id> --file requests.json

  # From stdin
  echo '[{"createSlide": {"insertionIndex": 1}}]' | gslides presentation batch-update <id> --stdin

  # Pretty JSON output
  gslides presentation batch-update <id> --file requests.json --pretty`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if batchUpdateFile == "" && !batchUpdateStdin {
			return fmt.Errorf("provide --file <requests.json> or --stdin")
		}
		if batchUpdateFile != "" && batchUpdateStdin {
			return fmt.Errorf("provide --file or --stdin, not both")
		}

		var data []byte
		var err error
		if batchUpdateFile != "" {
			data, err = os.ReadFile(batchUpdateFile)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
		} else {
			data, err = os.ReadFile("/dev/stdin")
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
		}

		var requests []*slides.Request
		if err := json.Unmarshal(data, &requests); err != nil {
			return fmt.Errorf("parsing requests JSON: %w", err)
		}
		if len(requests) == 0 {
			return fmt.Errorf("no requests found in JSON input")
		}

		resp, err := svc.Presentations.BatchUpdate(args[0], &slides.BatchUpdatePresentationRequest{
			Requests: requests,
		}).Do()
		if err != nil {
			return fmt.Errorf("batch update failed: %w", err)
		}

		if output.IsJSON(cmd) {
			return output.PrintJSON(resp, output.IsPretty(cmd))
		}
		fmt.Printf("Batch update applied: %d request(s) processed.\n", len(resp.Replies))
		fmt.Printf("Presentation ID: %s\n", resp.PresentationId)
		return nil
	},
}

func init() {
	presentationBatchUpdateCmd.Flags().StringVar(&batchUpdateFile, "file", "", "Path to JSON file containing array of Request objects")
	presentationBatchUpdateCmd.Flags().BoolVar(&batchUpdateStdin, "stdin", false, "Read requests JSON from stdin")

	presentationCmd.AddCommand(
		presentationCreateCmd,
		presentationGetCmd,
		presentationSlidesCmd,
		presentationBatchUpdateCmd,
	)
	rootCmd.AddCommand(presentationCmd)
}
