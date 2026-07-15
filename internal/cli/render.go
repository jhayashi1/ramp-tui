package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jhayashi1/ascii-tui/internal/engine"
	"github.com/jhayashi1/ascii-tui/internal/frames"
	"github.com/jhayashi1/ascii-tui/internal/library"
)

func init() {
	opts := engine.Options{}
	var output string
	var noColor bool

	renderCmd := &cobra.Command{
		Use:   "render <gif>",
		Short: "Pre-render a GIF into an ASCII animation frames file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Colored = !noColor
			anim, err := renderGif(args[0], opts, cmd.ErrOrStderr())
			if err != nil {
				return err
			}

			path := output
			if path == "" {
				dir, err := library.DefaultDir()
				if err != nil {
					return err
				}
				path, err = library.Save(dir, anim)
				if err != nil {
					return err
				}
			} else if err := saveTo(path, anim); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "rendered %d frames (%dx%d) to %s\n",
				len(anim.Frames), anim.Width, anim.Height, path)
			return nil
		},
	}

	renderCmd.Flags().StringVarP(&output, "output", "o", "", "output frames file (default: library)")
	renderCmd.Flags().IntVarP(&opts.Width, "width", "W", 0, "output width in characters")
	renderCmd.Flags().IntVarP(&opts.Height, "height", "H", 0, "output height in characters")
	renderCmd.Flags().BoolVar(&noColor, "no-color", false, "render without ANSI colors")
	renderCmd.Flags().BoolVar(&opts.Complex, "complex", false, "use a denser character ramp")
	renderCmd.Flags().StringVar(&opts.CustomRamp, "ramp", "", "custom character ramp, dark to bright")
	rootCmd.AddCommand(renderCmd)
}

func renderGif(gifPath string, opts engine.Options, progressOut io.Writer) (*frames.Animation, error) {
	f, err := os.Open(gifPath)
	if err != nil {
		return nil, fmt.Errorf("opening gif: %w", err)
	}
	defer f.Close()

	anim, err := engine.Render(f, opts, func(done, total int) {
		fmt.Fprintf(progressOut, "\rrendering frame %d/%d", done, total)
	})
	fmt.Fprintln(progressOut)
	if err != nil {
		return nil, err
	}
	anim.SourceName = filepath.Base(gifPath)
	return anim, nil
}

func saveTo(path string, anim *frames.Animation) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating frames file: %w", err)
	}
	if err := frames.Encode(f, anim); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
