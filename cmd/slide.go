package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/the20100/g-slides-cli/internal/api"
	"github.com/the20100/g-slides-cli/internal/output"
)

var slideCmd = &cobra.Command{
	Use:   "slide",
	Short: "Manage individual slides within a presentation",
}

// ---- slide get ----

var slideGetCmd = &cobra.Command{
	Use:   "get <presentation-id> <slide-id>",
	Short: "Get details of a specific slide",
	Long: `Get the full details of a specific slide (page) within a presentation.

Examples:
  gslides slide get <presentation-id> <slide-object-id>
  gslides slide get <presentation-id> <slide-object-id> --pretty`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		page, err := client.GetPage(args[0], args[1])
		if err != nil {
			return fmt.Errorf("getting slide: %w", err)
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(page, output.IsPretty(cmd))
		}
		output.PrintKeyValue([][]string{
			{"Slide ID", page.ObjectId},
			{"Page Type", page.PageType},
			{"Elements", fmt.Sprintf("%d", len(page.PageElements))},
		})
		if len(page.PageElements) > 0 {
			fmt.Println()
			fmt.Println("Elements:")
			headers := []string{"OBJECT ID", "TYPE", "DESCRIPTION"}
			rows := make([][]string, len(page.PageElements))
			for i, el := range page.PageElements {
				elType := elementType(el)
				rows[i] = []string{
					el.ObjectId,
					elType,
					output.Truncate(el.Description, 40),
				}
			}
			output.PrintTable(headers, rows)
		}
		return nil
	},
}

// ---- slide thumbnail ----

var (
	thumbnailWidth int64
	thumbnailMime  string
)

var slideThumbnailCmd = &cobra.Command{
	Use:   "thumbnail <presentation-id> <slide-id>",
	Short: "Get the thumbnail URL for a slide",
	Long: `Generate a thumbnail for a specific slide and return its URL.

Examples:
  gslides slide thumbnail <presentation-id> <slide-object-id>
  gslides slide thumbnail <presentation-id> <slide-id> --width 800
  gslides slide thumbnail <presentation-id> <slide-id> --mime PNG
  gslides slide thumbnail <presentation-id> <slide-id> --json`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		thumb, err := client.GetThumbnail(args[0], args[1], thumbnailWidth, thumbnailMime)
		if err != nil {
			return fmt.Errorf("getting thumbnail: %w", err)
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(thumb, output.IsPretty(cmd))
		}
		output.PrintKeyValue([][]string{
			{"Width", fmt.Sprintf("%d px", thumb.Width)},
			{"Height", fmt.Sprintf("%d px", thumb.Height)},
			{"URL", thumb.ContentUrl},
		})
		return nil
	},
}

// ---- slide add ----

var (
	slideAddIndex  int
	slideAddLayout string
)

var slideAddCmd = &cobra.Command{
	Use:   "add <presentation-id>",
	Short: "Add a new blank slide to a presentation",
	Long: `Add a new blank slide to a presentation at the specified position.

Examples:
  gslides slide add <presentation-id>
  gslides slide add <presentation-id> --index 2
  gslides slide add <presentation-id> --index 0 --layout BLANK`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		createSlide := map[string]interface{}{}
		if cmd.Flags().Changed("index") {
			createSlide["insertionIndex"] = slideAddIndex
		}
		if slideAddLayout != "" {
			createSlide["slideLayoutReference"] = map[string]string{
				"predefinedLayout": slideAddLayout,
			}
		}

		reqJSON, err := json.Marshal([]map[string]interface{}{
			{"createSlide": createSlide},
		})
		if err != nil {
			return fmt.Errorf("encoding request: %w", err)
		}

		resp, err := client.BatchUpdate(args[0], json.RawMessage(reqJSON))
		if err != nil {
			return fmt.Errorf("adding slide: %w", err)
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(resp, output.IsPretty(cmd))
		}

		// Try to extract the new slide's object ID from the reply
		if len(resp.Replies) > 0 {
			var reply struct {
				CreateSlide *struct {
					ObjectId string `json:"objectId"`
				} `json:"createSlide"`
			}
			if err := json.Unmarshal(resp.Replies[0], &reply); err == nil && reply.CreateSlide != nil {
				fmt.Printf("Slide added.\n")
				fmt.Printf("Slide ID: %s\n", reply.CreateSlide.ObjectId)
				return nil
			}
		}
		fmt.Println("Slide added.")
		return nil
	},
}

// ---- slide delete ----

var slideDeleteCmd = &cobra.Command{
	Use:   "delete <presentation-id> <slide-id>",
	Short: "Delete a slide from a presentation",
	Long: `Delete a slide from a presentation by its object ID.

Examples:
  gslides slide delete <presentation-id> <slide-object-id>`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		reqJSON, err := json.Marshal([]map[string]interface{}{
			{"deleteObject": map[string]string{"objectId": args[1]}},
		})
		if err != nil {
			return fmt.Errorf("encoding request: %w", err)
		}

		_, err = client.BatchUpdate(args[0], json.RawMessage(reqJSON))
		if err != nil {
			return fmt.Errorf("deleting slide: %w", err)
		}
		fmt.Printf("Slide %s deleted.\n", args[1])
		return nil
	},
}

// ---- slide duplicate ----

var slideDuplicateCmd = &cobra.Command{
	Use:   "duplicate <presentation-id> <slide-id>",
	Short: "Duplicate a slide within a presentation",
	Long: `Duplicate a slide within a presentation. The new slide is placed after the original.

Examples:
  gslides slide duplicate <presentation-id> <slide-object-id>
  gslides slide duplicate <presentation-id> <slide-id> --json`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		reqJSON, err := json.Marshal([]map[string]interface{}{
			{"duplicateObject": map[string]string{"objectId": args[1]}},
		})
		if err != nil {
			return fmt.Errorf("encoding request: %w", err)
		}

		resp, err := client.BatchUpdate(args[0], json.RawMessage(reqJSON))
		if err != nil {
			return fmt.Errorf("duplicating slide: %w", err)
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(resp, output.IsPretty(cmd))
		}

		if len(resp.Replies) > 0 {
			var reply struct {
				DuplicateObject *struct {
					ObjectId string `json:"objectId"`
				} `json:"duplicateObject"`
			}
			if err := json.Unmarshal(resp.Replies[0], &reply); err == nil && reply.DuplicateObject != nil {
				fmt.Printf("Slide duplicated.\n")
				fmt.Printf("New Slide ID: %s\n", reply.DuplicateObject.ObjectId)
				return nil
			}
		}
		fmt.Println("Slide duplicated.")
		return nil
	},
}

// ---- slide replace-text ----

var (
	replaceTextOld       string
	replaceTextNew       string
	replaceTextMatchCase bool
)

var slideReplaceTextCmd = &cobra.Command{
	Use:   "replace-text <presentation-id>",
	Short: "Replace all occurrences of text in a presentation",
	Long: `Replace all occurrences of a text string throughout the entire presentation.

Examples:
  gslides slide replace-text <id> --old "Hello" --new "Hi"
  gslides slide replace-text <id> --old "2023" --new "2024" --match-case`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if replaceTextOld == "" {
			return fmt.Errorf("--old is required")
		}

		reqJSON, err := json.Marshal([]map[string]interface{}{
			{
				"replaceAllText": map[string]interface{}{
					"containsText": map[string]interface{}{
						"text":      replaceTextOld,
						"matchCase": replaceTextMatchCase,
					},
					"replaceText": replaceTextNew,
				},
			},
		})
		if err != nil {
			return fmt.Errorf("encoding request: %w", err)
		}

		resp, err := client.BatchUpdate(args[0], json.RawMessage(reqJSON))
		if err != nil {
			return fmt.Errorf("replacing text: %w", err)
		}
		if output.IsJSON(cmd) {
			return output.PrintJSON(resp, output.IsPretty(cmd))
		}

		occurrences := int64(0)
		if len(resp.Replies) > 0 {
			var reply struct {
				ReplaceAllText *struct {
					OccurrencesChanged int64 `json:"occurrencesChanged"`
				} `json:"replaceAllText"`
			}
			if err := json.Unmarshal(resp.Replies[0], &reply); err == nil && reply.ReplaceAllText != nil {
				occurrences = reply.ReplaceAllText.OccurrencesChanged
			}
		}
		fmt.Printf("Replaced %s occurrences of %q with %q.\n",
			strconv.FormatInt(occurrences, 10), replaceTextOld, replaceTextNew)
		return nil
	},
}

func init() {
	// thumbnail flags
	slideThumbnailCmd.Flags().Int64Var(&thumbnailWidth, "width", 0, "Thumbnail width in pixels")
	slideThumbnailCmd.Flags().StringVar(&thumbnailMime, "mime", "", "Thumbnail MIME type: PNG or JPEG (default: PNG)")

	// add flags
	slideAddCmd.Flags().IntVar(&slideAddIndex, "index", 0, "Insertion index (0-based); appends at end if not set")
	slideAddCmd.Flags().StringVar(&slideAddLayout, "layout", "", "Predefined layout: BLANK, CAPTION_ONLY, TITLE, TITLE_AND_BODY, TITLE_AND_TWO_COLUMNS, TITLE_ONLY, SECTION_HEADER, SECTION_TITLE_AND_DESCRIPTION, ONE_COLUMN_TEXT, MAIN_POINT, BIG_NUMBER")

	// replace-text flags
	slideReplaceTextCmd.Flags().StringVar(&replaceTextOld, "old", "", "Text to search for (required)")
	slideReplaceTextCmd.Flags().StringVar(&replaceTextNew, "new", "", "Replacement text")
	slideReplaceTextCmd.Flags().BoolVar(&replaceTextMatchCase, "match-case", false, "Case-sensitive matching")

	slideCmd.AddCommand(
		slideGetCmd,
		slideThumbnailCmd,
		slideAddCmd,
		slideDeleteCmd,
		slideDuplicateCmd,
		slideReplaceTextCmd,
	)
	rootCmd.AddCommand(slideCmd)
}

// elementType returns a display name for a page element type.
func elementType(el *api.PageElement) string {
	if el.Shape != nil {
		return "shape"
	}
	if el.Image != nil {
		return "image"
	}
	if el.Table != nil {
		return "table"
	}
	if el.Video != nil {
		return "video"
	}
	if el.Line != nil {
		return "line"
	}
	if el.SheetsChart != nil {
		return "sheets-chart"
	}
	if el.ElementGroup != nil {
		return "group"
	}
	return "unknown"
}
